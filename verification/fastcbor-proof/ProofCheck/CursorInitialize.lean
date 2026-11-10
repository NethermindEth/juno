import ProofCheck.BigInitialize
import ProofCheck.Utf8Initialize
import Perennial.Proof.errors.New
import Perennial.Proof.errors.initialize
import Perennial.Proof.math.initialize
import Perennial.Proof.unsafe.initialize
import Perennial.Proof.cmp.initialize
import Perennial.Proof.math.bits.initialize
import Perennial.Proof.slices.initialize
import Perennial.Proof.strings.initialize

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

/-- Actual cursor package startup: allocates all five globals, executes the
    dependency initializers, calls the proved errors.New implementation, stores
    the returned objects, and retains them immutably. Already-initialized calls
    obtain the same resources through package.init's idempotence contract.

    Every dependency initializer is supplied by a checked proof. Runtime CPU
    startup is an explicit precondition, including arbitrary read-only feature
    flags in the initialization state. No initializer WP is assumed. -/
theorem wp_cursor_initialize (get_is_pkg_init : GoString → IProp GF)
    (Hinit : GetIsPkgInitProp pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor get_is_pkg_init) :
    {{ ownInitializing get_is_pkg_init ∗ PackageInitialized pkg_id.internal.cpu }}
      (App (Val cursor.initialize') (Val #()))
    {{ RET #(); ownInitializing get_is_pkg_init ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }} := by
  wp_start as ⟨Hown, #Hready⟩
  iapply wp_package_init (heq := Hinit.1) $$ [Hown] HΦ
  iframe Hown
  iintro Hown
  wp_auto
  wp_apply wp_GlobalAlloc (V := GoInterface) cursor.ErrUnsupported go.error as Hunsupported
  wp_apply wp_GlobalAlloc (V := GoInterface) cursor.ErrDepth go.error as Hdepth
  wp_apply wp_GlobalAlloc (V := GoInterface) cursor.ErrType go.error as Htype
  wp_apply wp_GlobalAlloc (V := GoInterface) cursor.ErrSyntax go.error as Hsyntax
  wp_apply wp_GlobalAlloc (V := GoInterface) cursor.ErrTruncated go.error as Htruncated
  wp_apply wp_big_initialize_ready _ Hinit.2.2.2.2.2.2.2.2.1 $$ [Hown] as ⟨Hown, #Hbig⟩
  · iframe Hown Hready
  wp_apply wp_utf8_initialize _ Hinit.2.2.2.2.2.2.2.1 $$ Hown as ⟨Hown, #Hutf8⟩
  wp_apply strings.initialize'.wp_proof _ Hinit.2.2.2.2.2.2.1 $$ Hown as ⟨Hown, #Hstrings⟩
  wp_apply (slices.initialize'.wp_proof
    (math.bits.initialize'.wp_proof «unsafe».initialize'.wp_proof) cmp.initialize'.wp_proof)
    _ Hinit.2.2.2.2.2.1 $$ Hown as ⟨Hown, #Hslices⟩
  wp_apply math.initialize'.wp_proof _ Hinit.2.2.2.2.1 $$ Hown as ⟨Hown, #Hmath⟩
  wp_apply (errors.initialize'.wp_proof errors.New.wp_proof)
    _ Hinit.2.2.2.1 $$ Hown as ⟨Hown, #Herrors⟩
  wp_apply «unsafe».initialize'.wp_proof _ Hinit.2.2.1 $$ Hown as ⟨Hown, #Hunsafe⟩
  wp_apply wp_binary_initialize _ Hinit.2.1 $$ Hown as ⟨Hown, #Hbinary⟩
  wp_apply errors.wp_New_errorString as %truncated #Etruncated
  ipersist Htruncated
  wp_apply errors.wp_New_errorString as %syntaxError #Esyntax
  ipersist Hsyntax
  wp_apply errors.wp_New_errorString as %typeError #Etype
  ipersist Htype
  wp_apply errors.wp_New_errorString as %depth #Edepth
  ipersist Hdepth
  iapply wp_fupd
  wp_apply errors.wp_New_errorString as %unsupported #Eunsupported
  ipersist Hunsupported
  ihave #Hcontents : CursorInitialized (GF := GF) $$ []
  · isimp only [CursorInitialized, errors.newVar]
    repeat (first | isplitr | iexists _ | iassumption)
  imodintro
  iframe Hown
  is_pkg_init_finish

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_cursor_initialize
