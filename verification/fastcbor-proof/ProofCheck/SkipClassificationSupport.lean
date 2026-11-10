import ProofCheck.HeadClassification

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Classification consumes a valid prefix with an exact remainder, or gives a
    rejection derivation for the complete remaining input. -/
def SkipClassified (d : Nat) (wire : List w8) : Prop :=
  (∃ item rest, wire = item ++ rest ∧ SkipValid d item) ∨
  (∃ offset reason, SkipFailure d wire offset reason)

/-- A failure at the current item is a failure with no enclosing context. -/
theorem skipFailure_here (d offset : Nat) (wire : List w8) (reason : SkipFailureReason)
    (failure : SkipFailureLeaf d wire offset reason) : SkipFailure d wire offset reason := by
  simpa only [List.nil_append, List.length_nil, Nat.zero_add] using
    (SkipFailure.within (SkipFailureContext.nil d) failure)

/-- A classified child failure extends through an array/map after valid siblings. -/
theorem skipFailure_container (d offset : Nat) (head : w8) (payload tail : List w8)
    (count perCount : w64) (items : List (List w8)) (reason : SkipFailureReason)
    (enc : HeadArgument head payload count)
    (hm : (head &&& W8 224 = W8 128 ∧ perCount = W64 1) ∨ (head &&& W8 224 = W8 160 ∧ perCount = W64 2))
    (bound : d < 32) (more : (items.length : Int) < uint.Z count * uint.Z perCount)
    (fit : uint.Z count * uint.Z perCount ≤ (items.flatten ++ tail).length)
    (valid : ∀ item ∈ items, SkipValid (d + 1) item)
    (failure : SkipFailure (d + 1) tail offset reason) :
    SkipFailure d ((head :: payload) ++ (items.flatten ++ tail))
      (1 + payload.length + items.flatten.length + offset) reason := by
  cases failure with
  | @within front tail leaf offset reason context failure =>
    have outer := SkipFailureContext.container items enc hm bound more fit valid context
    simpa only [List.append_assoc, List.length_append, List.length_cons, Nat.add_comm,
      Nat.add_left_comm, Nat.add_assoc] using SkipFailure.within outer failure

/-- A classified child failure extends through a supported tag, charging only
    an immediately tagged child as an additional nesting level. -/
theorem skipFailure_tag (d offset : Nat) (head next : w8) (payload tail : List w8)
    (argument : w64) (reason : SkipFailureReason)
    (enc : HeadArgument head payload argument) (hm : head &&& W8 224 = W8 192)
    (number : 3 < uint.Z argument) (bound : next &&& W8 224 = W8 192 → d < 32)
    (failure : SkipFailure (d + if next &&& W8 224 = W8 192 then 1 else 0) (next :: tail) offset reason) :
    SkipFailure d ((head :: payload) ++ (next :: tail)) (1 + payload.length + offset) reason := by
  generalize heq : next :: tail = wire at failure
  cases failure with
  | @within front rest leaf offset reason context failure =>
    have outer := SkipFailureContext.tag enc hm number heq.symm bound context
    simpa only [List.append_assoc, List.length_append, List.length_cons, Nat.add_comm,
      Nat.add_left_comm, Nat.add_assoc] using SkipFailure.within outer failure

/-- Any finite requested child sequence either has exactly that many accepted
    items or an accepted prefix followed by a classified failing remainder. -/
theorem skip_items_classify (d n : Nat) (wire : List w8)
    (child : ∀ rest : List w8, rest.length ≤ wire.length → SkipClassified d rest) :
    (∃ (items : List (List w8)) (rest : List w8), wire = items.flatten ++ rest ∧ items.length = n ∧ ∀ item ∈ items, SkipValid d item) ∨
    (∃ (items : List (List w8)) (tail : List w8) (offset : Nat) (reason : SkipFailureReason), wire = items.flatten ++ tail ∧ items.length < n ∧
      (∀ item ∈ items, SkipValid d item) ∧ SkipFailure d tail offset reason) := by
  induction n generalizing wire with
  | zero => exact Or.inl ⟨[], wire, rfl, rfl, by simp⟩
  | succ n ih =>
    rcases child wire (Nat.le_refl _) with ⟨item, rest, hsplit, valid⟩ | ⟨offset, reason, failure⟩
    · have hlen : rest.length ≤ wire.length := by rw [hsplit, List.length_append]; omega
      rcases ih rest (fun tail ht => child tail (by omega)) with
        ⟨items, tail, htail, hcount, children⟩ | ⟨items, tail, offset, reason, htail, hcount, children, failure⟩
      · refine Or.inl ⟨item :: items, tail, ?_, ?_, ?_⟩
        · simpa only [List.flatten_cons, List.append_assoc, htail] using hsplit
        · simp only [List.length_cons, hcount]
        · intro x hx
          rcases List.mem_cons.mp hx with rfl | hx
          · exact valid
          · exact children x hx
      · refine Or.inr ⟨item :: items, tail, offset, reason, ?_, ?_, ?_, failure⟩
        · simpa only [List.flatten_cons, List.append_assoc, htail] using hsplit
        · simp only [List.length_cons]; omega
        · intro x hx
          rcases List.mem_cons.mp hx with rfl | hx
          · exact valid
          · exact children x hx
    · exact Or.inr ⟨[], wire, offset, reason, rfl, by simp, by simp, failure⟩

/-- Container classification checks depth and count first, then classifies all
    children. Child classification is an explicit smaller-input premise. -/
theorem skip_container_classify (d : Nat) (head : w8) (payload body : List w8) (count perCount : w64)
    (enc : HeadArgument head payload count)
    (hm : (head &&& W8 224 = W8 128 ∧ perCount = W64 1) ∨ (head &&& W8 224 = W8 160 ∧ perCount = W64 2))
    (hdepth : d ≤ 32)
    (child : d < 32 → ∀ rest : List w8, rest.length ≤ body.length → SkipClassified (d + 1) rest) :
    SkipClassified d ((head :: payload) ++ body) := by
  by_cases bound : d < 32
  · by_cases short : body.length < uint.Z count * uint.Z perCount
    · exact Or.inr ⟨1 + payload.length, .count, skipFailure_here _ _ _ _ (.count enc hm bound short)⟩
    have fit : uint.Z count * uint.Z perCount ≤ body.length := by omega
    have nonneg : 0 ≤ uint.Z count * uint.Z perCount := by
      have h1 : 0 ≤ uint.Z count := by word
      have h2 : 0 ≤ uint.Z perCount := by word
      exact Int.mul_nonneg h1 h2
    let n := (uint.Z count * uint.Z perCount).toNat
    have hn : (n : Int) = uint.Z count * uint.Z perCount := by dsimp only [n]; omega
    rcases skip_items_classify (d + 1) n body (child bound) with
      ⟨items, rest, hsplit, hcount, valid⟩ | ⟨items, tail, offset, reason, hsplit, hcount, valid, failure⟩
    · refine Or.inl ⟨(head :: payload) ++ items.flatten, rest, ?_, ?_⟩
      · rw [hsplit, List.append_assoc]
      · rcases hm with ⟨hm, rfl⟩ | ⟨hm, rfl⟩
        · apply SkipValid.array items enc hm _ bound valid
          change (n : Int) = uint.Z count * 1 at hn
          omega
        · apply SkipValid.map items enc hm _ bound valid
          change (n : Int) = uint.Z count * 2 at hn
          omega
    · refine Or.inr ⟨1 + payload.length + items.flatten.length + offset, reason, ?_⟩
      rw [hsplit]
      exact skipFailure_container d offset head payload tail count perCount items reason enc hm bound
        (by omega) (by simpa only [hsplit] using fit) valid failure
  · have heq : d = 32 := by omega
    subst d
    exact Or.inr ⟨1 + payload.length, .depth, skipFailure_here _ _ _ _ (.containerDepth enc hm)⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.skipFailure_here
#print axioms Perennial.github_com.NethermindEth.juno.core.skipFailure_container
#print axioms Perennial.github_com.NethermindEth.juno.core.skipFailure_tag
#print axioms Perennial.github_com.NethermindEth.juno.core.skip_items_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.skip_container_classify
