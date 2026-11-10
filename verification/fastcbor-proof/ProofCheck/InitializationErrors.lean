import ProofCheck.FeltComplete

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The existing primitive/error contracts can obtain all five global values from
    the cursor package's initialization guarantee. Their values are the actual
    errorString pointers created by startup, not arbitrary supplied categories. -/
theorem cursor_initialized_felt_globals :
    isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ⊢
      ∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds := by
  iintro #Hpkg
  ihave Hcontents := cursor_initialized_errors $$ Hpkg
  isimp only [CursorInitialized, errors.newVar] at Hcontents
  icases Hcontents with ⟨⟨%truncated, #Htruncated, _⟩, ⟨%syntaxError, #Hsyntax, _⟩,
    ⟨%typeError, #Htype, _⟩, ⟨%depth, #Hdepth, _⟩, ⟨%unsupported, #Hunsupported, _⟩⟩
  iexists ({
    syntaxKind := interface.mkOk (go.PointerType errors.errorString.ty) #syntaxError,
    truncatedKind := interface.mkOk (go.PointerType errors.errorString.ty) #truncated,
    unsupportedKind := interface.mkOk (go.PointerType errors.errorString.ty) #unsupported,
    depthKind := interface.mkOk (go.PointerType errors.errorString.ty) #depth,
    typeKind := interface.mkOk (go.PointerType errors.errorString.ty) #typeError } : FeltErrorKinds)
  isimp only [FeltErrorGlobals, SkipErrorGlobals]
  iframe Hsyntax Htruncated Hunsupported Hdepth Htype

/-- Header callers obtain those globals through the schema's cursor dependency;
    final API contracts need not separately assume their initialization. -/
theorem header_initialized_felt_globals :
    isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ⊢
      ∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds := by
  iintro #Hpkg
  iapply cursor_initialized_felt_globals
  solve_pkg_init

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.cursor_initialized_felt_globals
#print axioms Perennial.github_com.NethermindEth.juno.core.header_initialized_felt_globals
