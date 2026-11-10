import ProofCheck.FeltSkipFailure

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

/-- Exhaustive outcomes for a present head and all remaining bytes. The supported
    case splits only its argument bytes, leaving the untouched remainder explicit. -/
inductive HeadClassified (head : w8) (tail : List w8) : Prop where
  | encoded (payload rest : List w8) (argument : w64)
      (split : tail = payload ++ rest) (encoding : HeadArgument head payload argument) : HeadClassified head tail
  | fault (reason : HeadError) (failure : HeadFault head reason) : HeadClassified head tail
  | truncated (size : w64)
      (info : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
      (sizeEq : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
      (short : tail.length < sint.Z size) : HeadClassified head tail

/-- Every byte and every remaining byte sequence has one of the independently
    specified supported, malformed/unsupported or incomplete head outcomes. -/
theorem head_classify (head : w8) (tail : List w8) : HeadClassified head tail := by
  by_cases hsmall : uint.Z (head &&& W8 31) < 24
  · exact .encoded [] tail _ rfl (.inline head hsmall)
  by_cases hbad : 27 < uint.Z (head &&& W8 31)
  · obtain ⟨reason, fault⟩ := headFault_classify head hbad
    exact .fault reason fault
  have hinfo : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27 := by omega
  by_cases h24 : head &&& W8 31 = W8 24
  · cases tail with
    | nil => exact .truncated (W64 1) hinfo (by rw [h24]; decide) (by decide)
    | cons b rest => exact .encoded [b] rest _ rfl (.byte head b h24)
  by_cases h25 : head &&& W8 31 = W8 25
  · cases tail with
    | nil => exact .truncated (W64 2) hinfo (by rw [h25]; decide) (by decide)
    | cons a rest =>
      cases rest with
      | nil => exact .truncated (W64 2) hinfo (by rw [h25]; decide) (by change (1 : Int) < 2; decide)
      | cons b rest => exact .encoded [a, b] rest _ rfl (.u16 head a b h25)
  by_cases h26 : head &&& W8 31 = W8 26
  · by_cases hlen : 4 ≤ tail.length
    · exact .encoded (tail.take 4) (tail.drop 4) _ (List.take_append_drop 4 tail).symm
        (.u32 head (tail.take 4) (by simp only [List.length_take, Nat.min_eq_left hlen]) h26)
    · exact .truncated (W64 4) hinfo (by rw [h26]; decide) (by change (tail.length : Int) < 4; omega)
  have h27 : head &&& W8 31 = W8 27 := by word
  by_cases hlen : 8 ≤ tail.length
  · exact .encoded (tail.take 8) (tail.drop 8) _ (List.take_append_drop 8 tail).symm
      (.u64 head (tail.take 8) (by simp only [List.length_take, Nat.min_eq_left hlen]) h27)
  · exact .truncated (W64 8) hinfo (by rw [h27]; decide) (by change (tail.length : Int) < 8; omega)

/-- The eight major types exhaust the possible high bits of every byte. -/
theorem byte_major_cases (head : w8) :
    head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32 ∨
    head &&& W8 224 = W8 64 ∨ head &&& W8 224 = W8 96 ∨
    head &&& W8 224 = W8 128 ∨ head &&& W8 224 = W8 160 ∨
    head &&& W8 224 = W8 192 ∨ head &&& W8 224 = W8 224 := by
  have h : ∀ head : w8,
    head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32 ∨
    head &&& W8 224 = W8 64 ∨ head &&& W8 224 = W8 96 ∨
    head &&& W8 224 = W8 128 ∨ head &&& W8 224 = W8 160 ∨
    head &&& W8 224 = W8 192 ∨ head &&& W8 224 = W8 224 := by decide
  exact h head

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.head_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.byte_major_cases
