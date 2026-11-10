import ProofCheck.FeltClassify
import ProofCheck.HeadDeterminism

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A complete head of the requested major type cannot simultaneously have any
    specified expectation error, including truncated extended arguments. -/
theorem expectedFailure_excludes_encoded (want head : w8) (payload rest wire : List w8)
    (argument : w64) (reason : ExpectedFailureReason)
    (enc : HeadArgument head payload argument) (major : head &&& W8 224 = want)
    (same : wire = (head :: payload) ++ rest) (failure : ExpectedFailure want wire reason) : False := by
  cases failure with
  | head tail fault =>
    simp only [List.cons_append, List.cons.injEq] at same
    obtain ⟨rfl, rfl⟩ := same
    have hi := headFault_info _ _ fault
    cases enc <;> simp_all <;> word
  | incomplete tail info sizeEq short =>
    simp only [List.cons_append, List.cons.injEq] at same
    obtain ⟨rfl, rfl⟩ := same
    cases enc with
    | inline small => omega
    | byte b infoEq =>
      rw [infoEq] at sizeEq
      have hs : sint.Z _ = 1 := congrArg sint.Z sizeEq.symm
      rw [hs] at short
      simp at short
      omega
    | u16 a b infoEq =>
      rw [infoEq] at sizeEq
      have hs : sint.Z _ = 2 := congrArg sint.Z sizeEq.symm
      rw [hs] at short
      simp at short
      omega
    | u32 bs hlen infoEq =>
      rw [infoEq] at sizeEq
      have hs : sint.Z _ = 4 := congrArg sint.Z sizeEq.symm
      rw [hs] at short
      simp [hlen] at short
      omega
    | u64 bs hlen infoEq =>
      rw [infoEq] at sizeEq
      have hs : sint.Z _ = 8 := congrArg sint.Z sizeEq.symm
      rw [hs] at short
      simp [hlen] at short
      omega
  | eof => simp at same
  | typeMismatch tail other mismatch notTag =>
    simp only [List.cons_append, List.cons.injEq] at same
    obtain ⟨rfl, _⟩ := same
    exact mismatch major
  | tag tail other mismatch isTag =>
    simp only [List.cons_append, List.cons.injEq] at same
    obtain ⟨rfl, _⟩ := same
    exact mismatch major

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.expectedFailure_excludes_encoded
