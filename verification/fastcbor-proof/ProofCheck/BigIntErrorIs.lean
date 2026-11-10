import ProofCheck.BigIntRootFailure
import Perennial.Proof.errors.Is

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The cursor error exposes its category through its actual Unwrap method. -/
theorem wp_cursorError_unwrap (err : Loc) (value : cursor.DecodeError) :
    {{ (err ↦ value : IProp GF) }}
      (App (Val (err @!! go.PointerType cursor.DecodeError.ty @!! go!"Unwrap")) (Val #()))
    {{ RET #value.Kind'; err ↦ value }} := by
  wp_start as Herr
  wp_auto
  wp_end

/-- Pointer error categories are tested by identity. The existing pinned Is
    model replaces reflection's comparable check by true; pointer targets meet
    that restriction. The DecodeError object is preserved. -/
theorem wp_cursorError_is_category (err kind target : Loc) (pos : w64) (reason : GoString) (args : GoSlice) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       err ↦ cursor.DecodeError.mk (interface.mkOk (go.PointerType errors.errorString.ty) #kind) pos reason args }}
      (App (App (Val (@! errors.Is))
        (Val #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err)))
        (Val #(interface.mkOk (go.PointerType errors.errorString.ty) #target)))
    {{ RET #(decide (kind = target));
       err ↦ cursor.DecodeError.mk (interface.mkOk (go.PointerType errors.errorString.ty) #kind) pos reason args }} := by
  wp_start as Herr
  wp_auto
  rw [show (go!"errors.is" : GoString) = errors.is' from rfl]
  wp_func_call
  wp_call
  wp_auto
  ihave HI : (∃ wrapped : Bool,
      "He" ∷ err_ptr ↦ (if wrapped then interface.mkOk (go.PointerType cursor.DecodeError.ty) #err
        else interface.mkOk (go.PointerType errors.errorString.ty) #kind) : IProp GF) $$ [err]
  · iexists true; iframe
  wp_for HI
  cases wrapped with
  | false =>
    wp_auto
    by_cases heq : kind = target
    · simp only [heq, _root_.decide_true]
      wp_auto
      wp_for_post
      wp_end
    · simp only [heq, _root_.decide_false]
      wp_auto
      go_type_set
      wp_auto
      go_type_set
      wp_auto
      go_type_set
      wp_auto
      wp_for_post
      wp_end
  | true =>
    wp_auto
    go_type_set
    wp_auto
    go_type_set
    wp_auto
    wp_apply wp_cursorError_unwrap err
      (cursor.DecodeError.mk (interface.mkOk (go.PointerType errors.errorString.ty) #kind) pos reason args)
      $$ [Herr] with Herr
    · iframe Herr
    wp_for_post
    iframe
    iexists false
    iframe

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_cursorError_unwrap
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_cursorError_is_category
