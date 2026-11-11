import ProofCheck.FeltValuesEmpty

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

/-- Null and undefined replace the old field with nil through the actual
    typed reader, preserving the remaining cursor fields and input. -/
theorem wp_readFeltValues_null (c dst : Loc) (old : GoSlice) (v : cursor.Cursor) (xs : List w8)
    (head : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some head) (hn : head = W8 246 ∨ head = W8 247) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltValues)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ slice.nil ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_limbSlice_null c v xs head dq hp hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

/-- The typed reader publishes a newly allocated empty slice, preserving
    the nil/empty distinction and exact head consumption. -/
theorem wp_readFeltValues_empty (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (dq : DFrac)
    (enc : HeadArgument head payload (W64 0)) (major : head &&& W8 224 = W8 128)
    (hdepth : sint.Z v.depth' < 32) (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltValues)) (Val #c)) (Val #dst))
    {{ (out : GoSlice), RET #interface.nil; dst ↦ out ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       out ↦* ([] : List (GoArray w64 4)) ∗ ownSliceCap (GoArray w64 4) out (DFrac.own 1) ∗
       ⌜out.ptr ≠ null⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_limbSlice_empty c v xs payload rem head dq enc major hdepth hp hbound hi hbytes
    $$ [Hc Hs] as %out ⟨Hc, Hs, Hout, Hcap, %Hnn⟩
  · iframe Hc Hs
  wp_end

/-- Outer-head errors preserve the old typed field and full cursor, with
    the actual diagnostic propagated unchanged. -/
theorem wp_readFeltValues_outer_failure (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
    (failure : ExpectedFailure (W8 128) wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltValues)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 128) reason kind }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_limbSlice_outer_failure c v xs wire dq reason kind hn hu failure hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Null and undefined replace the old field with nil through the actual
    typed reader, preserving the remaining cursor fields and input. -/
theorem wp_readPlainFeltValues_null (c dst : Loc) (old : GoSlice) (v : cursor.Cursor) (xs : List w8)
    (head : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some head) (hn : head = W8 246 ∨ head = W8 247) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadPlainFeltValues)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ slice.nil ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_limbSlice_null c v xs head dq hp hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

/-- The typed reader publishes a newly allocated empty slice, preserving
    the nil/empty distinction and exact head consumption. -/
theorem wp_readPlainFeltValues_empty (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (dq : DFrac)
    (enc : HeadArgument head payload (W64 0)) (major : head &&& W8 224 = W8 128)
    (hdepth : sint.Z v.depth' < 32) (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadPlainFeltValues)) (Val #c)) (Val #dst))
    {{ (out : GoSlice), RET #interface.nil; dst ↦ out ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       out ↦* ([] : List (GoArray w64 4)) ∗ ownSliceCap (GoArray w64 4) out (DFrac.own 1) ∗
       ⌜out.ptr ≠ null⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_limbSlice_empty c v xs payload rem head dq enc major hdepth hp hbound hi hbytes
    $$ [Hc Hs] as %out ⟨Hc, Hs, Hout, Hcap, %Hnn⟩
  · iframe Hc Hs
  wp_end

/-- Outer-head errors preserve the old typed field and full cursor, with
    the actual diagnostic propagated unchanged. -/
theorem wp_readPlainFeltValues_outer_failure (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
    (failure : ExpectedFailure (W8 128) wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadPlainFeltValues)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 128) reason kind }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_limbSlice_outer_failure c v xs wire dq reason kind hn hu failure hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltValues_null
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltValues_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltValues_outer_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readPlainFeltValues_null
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readPlainFeltValues_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readPlainFeltValues_outer_failure
