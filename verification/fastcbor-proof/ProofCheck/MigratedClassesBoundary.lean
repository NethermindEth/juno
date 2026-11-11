import ProofCheck.FeltMapOwnership
import ProofCheck.MapExpectedFailure
import ProofCheck.Errors

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 4096

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The actual MigratedClasses reader publishes nil for null/undefined,
    consumes one byte and leaves depth unchanged. -/
theorem wp_readMigratedClasses_null (c dst : Loc) (old : Loc) (v : cursor.Cursor) (xs : List w8)
    (head : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some head) (hn : head = W8 246 ∨ head = W8 247) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadMigratedClasses)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ map.nil ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_at c v xs head dq hp hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

/-- Every rejected outer map head is propagated with the original cursor and
    actual owned diagnostic, before allocation and depth checks. -/
theorem wp_readMigratedClasses_outer_failure (c dst : Loc) (old : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
    (failure : ExpectedFailure (W8 160) wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadMigratedClasses)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err); dst ↦ old ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 160) reason kind }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_not_nullish c v xs wire dq hn hu hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_MapLen_expected_failure c v xs wire dq reason kind failure hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Outer count rejection preserves the original destination, cursor and depth. -/
theorem wp_readMigratedClasses_count (c dst : Loc) (old : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (hmajor : head &&& W8 224 = W8 160)
    (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hcount : sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length) < 2 * uint.Z argument)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadMigratedClasses)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err); dst ↦ old ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"count exceeds the input") slice.nil }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  have hn : head ≠ W8 246 ∧ head ≠ W8 247 := by
    constructor <;> intro eq <;> subst head <;> contradiction
  wp_auto
  wp_apply wp_null_other c v xs head dq hp hi hn.1 hn.2 $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_MapLen_count c v xs payload rem head argument dq kind enc hmajor hp hbound hcount hi hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Outer depth rejection preserves the original destination, cursor and depth. -/
theorem wp_readMigratedClasses_depth (c dst : Loc) (old : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (hmajor : head &&& W8 224 = W8 160)
    (hp : 0 ≤ sint.Z v.pos') (hdepth : 32 ≤ sint.Z v.depth')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hcount : 2 * uint.Z argument ≤ sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length))
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr cursor.ErrDepth ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadMigratedClasses)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err); dst ↦ old ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"nested more than %d levels") args ∗
       args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  have hn : head ≠ W8 246 ∧ head ≠ W8 247 := by
    constructor <;> intro eq <;> subst head <;> contradiction
  wp_auto
  wp_apply wp_null_other c v xs head dq hp hi hn.1 hn.2 $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_MapLen_depth c v xs payload rem head argument dq kind enc hmajor hp hdepth hbound hcount hi hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readMigratedClasses_null
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readMigratedClasses_outer_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readMigratedClasses_count
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readMigratedClasses_depth
