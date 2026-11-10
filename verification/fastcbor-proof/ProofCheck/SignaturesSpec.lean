import ProofCheck.FeltPointersComplete

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- One signature's nullable pointer slice, independently of its allocation. -/
abbrev SignatureValue := Option (List (Option (GoArray w64 4)))

structure SignatureWire (depth : Nat) where
  bytes : List w8
  value : SignatureValue
  valid : FeltPointersValid depth bytes value

/-- Each slice header owns its corresponding pointer table and felt backing
    storage separately. Nil child slices carry no allocation requirement. -/
def SignatureChildren (slices : List GoSlice) (values : List SignatureValue) : IProp GF :=
  iprop([∗list] child;value ∈ slices;values, FeltPointersResult child value)

/-- The outer slice owns its headers and all nested signature allocations. An
    allocated empty outer array remains distinguishable from a nil result. -/
def SignaturesOwned (out : GoSlice) (values : List SignatureValue) : IProp GF := iprop(
  ∃ slices : List GoSlice, out ↦* slices ∗ SignatureChildren slices values ∗ ⌜out.ptr ≠ null⌝)

inductive SignaturesValid : Nat → List w8 → Option (List SignatureValue) → Prop where
  | nullish {d : Nat} {head : w8} (hn : head = W8 246 ∨ head = W8 247) :
      SignaturesValid d [head] none
  | array {d : Nat} {head : w8} {payload : List w8} {count : w64}
      (items : List (SignatureWire (d + 1))) (enc : HeadArgument head payload count)
      (major : head &&& W8 224 = W8 128) (size : uint.Z count = items.length) (bound : d < 32) :
      SignaturesValid d ((head :: payload) ++ items.flatMap SignatureWire.bytes) (some (items.map SignatureWire.value))

def SignaturesResult (out : GoSlice) (value : Option (List SignatureValue)) : IProp GF :=
  match value with
  | none => iprop(⌜out = slice.nil⌝)
  | some values => SignaturesOwned out values

theorem signatureWire_nonempty (d : Nat) (item : SignatureWire d) : 0 < item.bytes.length := by
  rcases item with ⟨bytes, value, valid⟩
  cases valid <;> simp only [List.length_cons, List.length_nil, List.length_append] <;> omega

theorem signatureWire_count_bound (d : Nat) (items : List (SignatureWire d)) :
    items.length ≤ (items.flatMap SignatureWire.bytes).length := by
  induction items with
  | nil => simp
  | cons item items ih =>
    have := signatureWire_nonempty d item
    simp only [List.length_cons, List.flatMap_cons, List.length_append]; omega

theorem signatureChildren_nil : ⊢ (SignatureChildren [] [] : IProp GF) := by
  unfold SignatureChildren
  iapply BigSepL2.bigSepL2_nil.2
  itrivial

theorem signatureChildren_length (slices : List GoSlice) (values : List SignatureValue) :
    SignatureChildren slices values ⊢ (⌜slices.length = values.length⌝ : IProp GF) := by
  unfold SignatureChildren
  exact BigSepL2.bigSepL2_length

theorem signatureChildren_snoc (slices : List GoSlice) (values : List SignatureValue)
    (child : GoSlice) (value : SignatureValue) :
    SignatureChildren slices values ∗ FeltPointersResult child value ⊢
      (SignatureChildren (slices ++ [child]) (values ++ [value]) : IProp GF) := by
  unfold SignatureChildren
  iintro ⟨Hchildren, Hchild⟩
  iapply BigSepL2.bigSepL2_snoc.2
  iframe Hchildren Hchild

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.signatureWire_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.signatureWire_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.signatureChildren_nil
#print axioms Perennial.github_com.NethermindEth.juno.core.signatureChildren_length
#print axioms Perennial.github_com.NethermindEth.juno.core.signatureChildren_snoc
