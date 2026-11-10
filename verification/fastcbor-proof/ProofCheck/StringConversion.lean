import ProofCheck.Payload
import Perennial.Golang.Theory.String

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics]

/-- The exact byte-slice-to-string conversion used after successful validation
    returns the payload bytes and restores ownership of the whole original input.
    Conversion itself imposes no UTF-8 restriction; validation is a separate call. -/
theorem wp_bytes_to_string_range (data : GoSlice) (xs body : List w8)
    (low high : w64) (dq : DFrac)
    (hbound : 0 ≤ sint.Z low ∧ sint.Z low ≤ sint.Z high ∧ sint.Z high ≤ sint.Z data.len)
    (hbytes : subslice (sint.nat low) (sint.nat high) xs = body) :
    {{ (data ↦*{dq} xs : IProp GF) }}
      (App (Val (GoInstruction (Convert (go.SliceType go.byte) go.string)))
        (Val #(slice.slice data w8 low high)))
    {{ RET #body; data ↦*{dq} xs }} := by
  iintro %Φ Hs HΦ
  icases (ownSlice_slice low high data dq xs hbound).1 $$ Hs with ⟨Hpre, Hbody, Hpost⟩
  isimp only [hbytes] at Hbody
  wp_apply wp_bytes_to_string (slice.slice data w8 low high) body dq $$ [Hbody] with Hbody
  · iframe Hbody
  iapply HΦ
  iapply (ownSlice_slice low high data dq xs hbound).2
  rw [hbytes]
  iframe Hpre Hbody Hpost

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bytes_to_string_range
