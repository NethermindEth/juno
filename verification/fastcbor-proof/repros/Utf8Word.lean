-- Expected to fail at idx_if with reversed IndexRef operands on perennial.rev.
-- Kept outside the audited proof target; see README.md, generic slice-index proposal.
import ProofCheck.Payload
import ProofCheck.GeneratedProof.unicode.utf8
import Perennial.Proof.encoding.binary.Base

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 1600000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
open encoding.binary
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The actual generic UTF-8 word helper, instantiated at []byte on the pinned 64-bit target. -/
theorem wp_utf8_word (data : GoSlice) (bs rem : List w8) (dq : DFrac) (hlen : bs.length = 8) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.unicode.utf8 ∗ data ↦*{dq} (bs ++ rem) }}
      (App (App (Val (GoInstruction (FuncResolve unicode.utf8.word [go.SliceType go.byte]))) (Val #())) (Val #data))
    {{ RET #(leToU64 bs); data ↦*{dq} (bs ++ rem) }} := by
  obtain ⟨w0, w1, w2, w3, w4, w5, w6, w7, rfl⟩ :
      ∃ w0 w1 w2 w3 w4 w5 w6 w7, bs = [w0, w1, w2, w3, w4, w5, w6, w7] := by
    match bs, hlen with
    | [w0, w1, w2, w3, w4, w5, w6, w7], _ => exact ⟨w0, w1, w2, w3, w4, w5, w6, w7, rfl⟩
  wp_start as Hs
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  (try wp_auto)
  idx_if
  wp_apply wp_load_slice_index data _ _ dq w0 (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; rfl
  (try wp_auto)
  idx_if
  wp_apply wp_load_slice_index data _ _ dq w1 (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; rfl
  (try wp_auto)
  idx_if
  wp_apply wp_load_slice_index data _ _ dq w2 (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; rfl
  (try wp_auto)
  idx_if
  wp_apply wp_load_slice_index data _ _ dq w3 (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; rfl
  (try wp_auto)
  idx_if
  wp_apply wp_load_slice_index data _ _ dq w4 (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; rfl
  (try wp_auto)
  idx_if
  wp_apply wp_load_slice_index data _ _ dq w5 (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; rfl
  (try wp_auto)
  idx_if
  wp_apply wp_load_slice_index data _ _ dq w6 (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; rfl
  (try wp_auto)
  idx_if
  wp_apply wp_load_slice_index data _ _ dq w7 (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; rfl
  rw [encoding.binary.leToU64_8]
  iapply HΦ $$ Hs

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_utf8_word
