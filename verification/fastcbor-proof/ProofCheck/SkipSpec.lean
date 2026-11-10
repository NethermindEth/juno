import ProofCheck.SkipReject

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 1600000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]


/-- Accepted ignored values, independent of the scanner's shortcut and loop structure.
Arrays/maps open one level. Only a tag immediately containing another tag opens a level.
Text bytes are intentionally not constrained to UTF-8; tags 0 through 3 are excluded. -/
inductive SkipValid : Nat → List w8 → Prop where
  | atom {depth : Nat} {head : w8} {payload body : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (accepted : SkipAtom head argument body) :
      SkipValid depth ((head :: payload) ++ body)
  | array {depth : Nat} {head : w8} {payload : List w8} {argument : w64}
      (items : List (List w8)) (enc : HeadArgument head payload argument)
      (major : head &&& W8 224 = W8 128) (count : uint.Z argument = items.length)
      (bound : depth < 32) (children : ∀ item ∈ items, SkipValid (depth + 1) item) :
      SkipValid depth ((head :: payload) ++ items.flatten)
  | map {depth : Nat} {head : w8} {payload : List w8} {argument : w64}
      (items : List (List w8)) (enc : HeadArgument head payload argument)
      (major : head &&& W8 224 = W8 160) (count : 2 * uint.Z argument = items.length)
      (bound : depth < 32) (children : ∀ item ∈ items, SkipValid (depth + 1) item) :
      SkipValid depth ((head :: payload) ++ items.flatten)
  | tag {depth : Nat} {head next : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 192)
      (number : 3 < uint.Z argument) (bound : next &&& W8 224 = W8 192 → depth < 32)
      (child : SkipValid (depth + if next &&& W8 224 = W8 192 then 1 else 0) (next :: tail)) :
      SkipValid depth ((head :: payload) ++ (next :: tail))

/-- Every accepted item contains its head byte. -/
theorem skipValid_nonempty (depth : Nat) (wire : List w8) (valid : SkipValid depth wire) :
    0 < wire.length := by
  cases valid <;> simp [List.length_append]

/-- A uint64 head always occupies exactly nine bytes, regardless of its payload bits. -/
theorem skipValid_wide_uint (depth : Nat) (wire : List w8) (valid : SkipValid depth wire)
    (hi : wire[0]? = some (W8 27)) : wire.length = 9 := by
  cases valid with
  | @atom depth head payload body argument enc accepted =>
    have heq : head = W8 27 := by simpa using hi
    subst head
    cases accepted with
    | integer hm =>
      cases enc with
      | inline h => contradiction
      | byte _ h => contradiction
      | u16 _ _ h => contradiction
      | u32 _ _ h => contradiction
      | u64 _ hlen _ => simp [hlen]
    | simple hm _ => contradiction
    | bytes hm _ _ => rcases hm with h | h <;> contradiction
  | @array depth head payload argument items enc hm count bound children =>
    have heq : head = W8 27 := by simpa using hi
    rw [heq] at hm
    contradiction
  | @map depth head payload argument items enc hm count bound children =>
    have heq : head = W8 27 := by simpa using hi
    rw [heq] at hm
    contradiction
  | @tag depth head next payload tail argument enc hm number bound child =>
    have heq : head = W8 27 := by simpa using hi
    rw [heq] at hm
    contradiction

/-- The byte checks made by both production skip shortcuts. -/
def SkipWideShape (wire : List w8) : Prop :=
  wire[0]? = some (W8 132) ∧ wire[1]? = some (W8 27) ∧ wire[10]? = some (W8 27) ∧
  wire[19]? = some (W8 27) ∧ wire[28]? = some (W8 27)

/-- A shortcut match on a valid ignored value consumes precisely that value, never its suffix. -/
theorem skipValid_wide_length (depth : Nat) (wire rem : List w8) (valid : SkipValid depth wire)
    (shape : SkipWideShape (wire ++ rem)) : wire.length = 37 := by
  obtain ⟨h0, h1, h10, h19, h28⟩ := shape
  cases valid with
  | @atom depth head payload body argument enc accepted =>
    have heq : head = W8 132 := by simpa using h0
    exact False.elim ((skipAtom_not_felt _ _ _ accepted) heq)
  | @array depth head payload argument items enc hm hcount hdepth children =>
    have heq : head = W8 132 := by simpa using h0
    subst head
    obtain ⟨rfl, ha⟩ := headArgument_inline_inv (W8 132) payload argument enc (by decide)
    have hlen : items.length = 4 := by rw [ha] at hcount; change (4 : Int) = items.length at hcount; omega
    obtain ⟨a, b, c, d, rfl⟩ : ∃ a b c d : List w8, items = [a, b, c, d] :=
      ⟨items[0]'(by omega), items[1]'(by omega), items[2]'(by omega), items[3]'(by omega),
        List.eq_getElem_of_length_eq_four items hlen⟩
    have va := children a (by simp)
    have vb := children b (by simp)
    have vc := children c (by simp)
    have vd := children d (by simp)
    have hpa := skipValid_nonempty _ _ va
    have hpb := skipValid_nonempty _ _ vb
    have hpc := skipValid_nonempty _ _ vc
    have hpd := skipValid_nonempty _ _ vd
    have hia : a[0]? = some (W8 27) := by
      simpa [List.getElem?_append, hpa] using h1
    have hla := skipValid_wide_uint _ _ va hia
    have hib : b[0]? = some (W8 27) := by
      simpa [List.getElem?_append, hla, hpb] using h10
    have hlb := skipValid_wide_uint _ _ vb hib
    have hic : c[0]? = some (W8 27) := by
      simpa [List.getElem?_append, hla, hlb, hpc] using h19
    have hlc := skipValid_wide_uint _ _ vc hic
    have hid : d[0]? = some (W8 27) := by
      simpa [List.getElem?_append, hla, hlb, hlc, hpd] using h28
    have hld := skipValid_wide_uint _ _ vd hid
    simp [hla, hlb, hlc, hld]
  | @map depth head payload argument items enc hm count bound children =>
    have heq : head = W8 132 := by simpa using h0
    rw [heq] at hm
    contradiction
  | @tag depth head next payload tail argument enc hm number bound child =>
    have heq : head = W8 132 := by simpa using h0
    rw [heq] at hm
    contradiction

/-- The wide shape is an array and therefore requires one available nesting level. -/
theorem skipValid_wide_depth (depth : Nat) (wire rem : List w8) (valid : SkipValid depth wire)
    (shape : SkipWideShape (wire ++ rem)) : depth < 32 := by
  have h0 := shape.1
  cases valid with
  | @atom depth head payload body argument enc accepted =>
    have heq : head = W8 132 := by simpa using h0
    exact False.elim ((skipAtom_not_felt _ _ _ accepted) heq)
  | array _ _ _ _ bound _ => exact bound
  | @map depth head payload argument items enc hm count bound children =>
    have heq : head = W8 132 := by simpa using h0
    rw [heq] at hm
    contradiction
  | @tag depth head next payload tail argument enc hm number bound child =>
    have heq : head = W8 132 := by simpa using h0
    rw [heq] at hm
    contradiction

/-- A valid sequence takes at least one byte per item, justifying the scanner's count check. -/
theorem skipValid_items_bound (depth : Nat) (items : List (List w8))
    (children : ∀ item ∈ items, SkipValid depth item) : items.length ≤ items.flatten.length := by
  induction items with
  | nil => simp
  | cons item items ih =>
    have hi := skipValid_nonempty depth item (children item (by simp))
    have hs := ih (by intro x hx; exact children x (by simp [hx]))
    simp only [List.length_cons, List.flatten_cons, List.length_append]
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.skipValid_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.skipValid_wide_uint
#print axioms Perennial.github_com.NethermindEth.juno.core.skipValid_wide_length

#print axioms Perennial.github_com.NethermindEth.juno.core.skipValid_wide_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.skipValid_items_bound
