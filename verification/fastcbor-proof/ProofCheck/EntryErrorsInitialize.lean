import ProofCheck.BinaryInitialize
import Perennial.Proof.fmt.initialize

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

/-- Complete actual startup of the shared entry-error package, including its
    errors/fmt dependencies, both errors.New calls and immutable global stores.
    No planner or core initialization premise is needed. -/
theorem wp_entryErrors_initialize :
    DecoderDependencyInit (GF := GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr cborerr.initialize' := by
  intro g Hinit
  have herrors := errors.initialize'.wp_proof (GF := GF) errors.New.wp_proof
  have hsync := sync.initialize'.wp_proof (GF := GF)
    internal.synctest.initialize'.wp_proof internal.race.initialize'.wp_proof sync.atomic.initialize'.wp_proof
  have hfmt := fmt.initialize'.wp_proof hsync (io.initialize'.wp_proof hsync herrors) herrors errors.New.wp_proof
  wp_start as Hown
  iapply wp_package_init (heq := Hinit.1) $$ [Hown] HΦ
  iframe Hown
  iintro Hown
  wp_auto
  wp_apply wp_GlobalAlloc (V := GoInterface) cborerr.ErrExtraneousData go.error as Hextra
  wp_apply wp_GlobalAlloc (V := GoInterface) cborerr.ErrUnsupported go.error as Hunsupported
  wp_apply hfmt _ Hinit.2.2.1 $$ Hown as ⟨Hown, #Hfmt⟩
  wp_apply herrors _ Hinit.2.1 $$ Hown as ⟨Hown, #Herrors⟩
  wp_apply errors.wp_New_errorString as %unsupported #Eunsupported
  ipersist Hunsupported
  iapply wp_fupd
  wp_apply errors.wp_New_errorString as %extra #Eextra
  ipersist Hextra
  ihave #Hcontents : EntryErrorsInitialized (GF := GF) $$ []
  · isimp only [EntryErrorsInitialized, errors.newVar]
    repeat (first | isplitr | iexists _ | iassumption)
  imodintro
  iframe Hown
  is_pkg_init_finish

/-- The two fixed error objects are recovered from completed initialization. -/
theorem entryErrors_initialized_objects :
    isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ⊢
      EntryErrorsInitialized (GF := GF) := by
  exact isPkgInit_access _

/-- The actual shared error method exposes its stored category without mutation;
    diagnostic formatting remains outside the decoder proof scope. -/
theorem wp_entryError_unwrap (err : Loc) (value : cborerr.DecodeError) :
    {{ (err ↦ value : IProp GF) }}
      (App (Val (err @!! go.PointerType cborerr.DecodeError.ty @!! go!"Unwrap")) (Val #()))
    {{ RET #value.Kind'; err ↦ value }} := by
  wp_start as Herr
  wp_auto
  wp_end

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_entryErrors_initialize
#print axioms Perennial.github_com.NethermindEth.juno.core.entryErrors_initialized_objects

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_entryError_unwrap
