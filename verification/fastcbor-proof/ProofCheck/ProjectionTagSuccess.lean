import ProofCheck.ProjectionTagComplete

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

/-- A successful tag prelude has a concrete nil return, exact consumed prefix
    and unchanged depth. This includes both allowed tags and untagged input. -/
theorem wp_projectionTag_success (c : Loc) (v : cursor.Cursor) (xs wire rest : List w8)
    (dq : DFrac) (consumed : Nat) (kinds : FeltErrorKinds)
    (spec : ProjectionTagSpec wire consumed rest .success)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       FeltErrorGlobals kinds ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (@! cborReadProjectionTag)) (Val #c))
    {{ RET #interface.nil;
       c ↦ {v with pos' := v.pos' + W64 consumed} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 consumed)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start_folded as ⟨Hglobals, Hc, Hs⟩
  wp_apply wp_projectionTag_spec c v xs wire rest dq consumed .success kinds spec hp hbytes
    $$ [Hglobals Hc Hs] as %result ⟨Hc, Hs, Herror, %Hnext⟩
  · iframe Hglobals Hc Hs
  isimp only [ProjectionTagError] at Herror
  icases Herror with %Hresult
  subst result
  wp_end

/-- An initialized unsupported category is a concrete errorString interface,
    hence non-nil. Agreement ties any supplied globals to the initialized one. -/
theorem projection_unsupported_nonNil (kinds : FeltErrorKinds) :
    isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
      FeltErrorGlobals (GF := GF) kinds ⊢ ⌜kinds.unsupportedKind ≠ interface.nil⌝ := by
  iintro ⟨#Hpkg, Hglobals⟩
  ihave Herrors := cursor_initialized_errors $$ Hpkg
  isimp only [CursorInitialized, CursorErrorGlobal] at Herrors
  icases Herrors with ⟨_, _, _, _, ⟨%p, #Hcanonical, _⟩⟩
  isimp only [FeltErrorGlobals, SkipErrorGlobals] at Hglobals
  icases Hglobals with ⟨⟨_, _, Hunsupported, _⟩, _⟩
  ihave %agree := typedPointsto_agree_wand (globalAddr cursor.ErrUnsupported) DFrac.discard DFrac.discard
    (interface.mkOk (go.PointerType errors.errorString.ty) #p) kinds.unsupportedKind $$ Hcanonical Hunsupported
  ipureintro
  rw [← agree]
  simp

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_projectionTag_success
#print axioms Perennial.github_com.NethermindEth.juno.core.projection_unsupported_nonNil
