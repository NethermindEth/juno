import ProofCheck.SkipTaggedBoundary

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

/-- Container iteration checks depth before count or child access. -/
theorem wp_skipElements_depth (c : Loc) (v : cursor.Cursor)
    (count limit perCount : w64) (kind : GoInterface) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrDepth ↦□ kind ∗ c ↦ v }}
      (App (App (App (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skipElements"))
        (Val #count)) (Val #limit)) (Val #perCount)) (Val #(W64 32)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ ∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind v.pos'
         (go!"nested more than %d levels") args ∗ args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start as ⟨#Hkind, Hc⟩
  wp_auto
  wp_apply wp_errDepth v.pos' kind as %err Herr
  · iframe Hkind
  wp_end

/-- Oversized counts are rejected before multiplication and before any child read. -/
theorem wp_skipElements_count (c : Loc) (v : cursor.Cursor)
    (count limit perCount depth : w64) (kind : GoInterface)
    (hd : 0 ≤ sint.Z depth ∧ sint.Z depth < 32) (hcount : uint.Z limit < uint.Z count) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v }}
      (App (App (App (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skipElements"))
        (Val #count)) (Val #limit)) (Val #perCount)) (Val #depth))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"count exceeds the input") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc⟩
  have hdep : ¬ sint.Z (W64 32) < sint.Z (depth + W64 1) := by word
  wp_auto
  simp only [hdep, _root_.decide_false]
  wp_auto
  simp only [hcount, _root_.decide_true]
  wp_auto
  wp_apply wp_fail c v kind (go!"count exceeds the input") slice.nil $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skipElements_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skipElements_count
