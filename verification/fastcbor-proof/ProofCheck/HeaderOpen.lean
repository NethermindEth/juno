import ProofCheck.HeaderOwnership
import ProofCheck.StructOpenComplete

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Every opening error returns before Header field dispatch, preserving the
    destination, original cursor/depth and input with the exact owned diagnostic. -/
theorem wp_readHeader_open_failure (c dst fields : Loc) (old : Header) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (reason : FeltFailureReason) (kind : GoInterface)
    (failure : StructOpenFailure d wire reason) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ feltFailureAddr (GF := GF) reason ↦□ kind ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadHeader)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ FeltFailureError err v.pos' reason kind }} := by
  wp_start as ⟨#Hfields, #Hkind, Hdst, Hc, Hs⟩
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_failure c fields v xs wire dq d reason kind failure hd hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_for
  wp_end

/-- Null/undefined preserve the existing Header fields, consume one byte,
    and leave the caller's depth unchanged. They never access field metadata. -/
theorem wp_readHeader_nullish (c dst fields : Loc) (old : Header) (v : cursor.Cursor)
    (xs : List w8) (head : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some head) (hn : head = W8 246 ∨ head = W8 247) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadHeader)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ old ∗ c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨#Hfields, Hdst, Hc, Hs⟩
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_nullish c fields v xs head dq hp hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_for
  wp_apply wp_nextField_done c _ fields $$ [s] with Hst
  · iframe
  wp_for_post
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readHeader_open_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readHeader_nullish
