import ProofCheck.GasPriceOpen
import ProofCheck.StructOpenSpec

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

/-- GasPrice and Header use the same independent record-opening relation. -/
abbrev GasPriceOpenFailure := StructOpenFailure

theorem gasPrice_open_classify (d : Nat) (wire : List w8) (hdepth : d ≤ 32) :
    (∃ head tail, wire = head :: tail ∧ (head = W8 246 ∨ head = W8 247)) ∨
    (∃ head payload body count, wire = (head :: payload) ++ body ∧
      HeadArgument head payload count ∧ head &&& W8 224 = W8 160 ∧ d < 32 ∧
      2 * uint.Z count ≤ body.length) ∨
    (∃ reason, GasPriceOpenFailure d wire reason) := by
  exact struct_open_classify d wire hdepth

/-- Every specified opening failure is realized by the actual generated reader,
    preserving its destination, original cursor/depth and entire input. -/
theorem wp_readGasPrice_open_failure (c dst fields : Loc) (old : GasPrice) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (reason : FeltFailureReason) (kind : GoInterface)
    (failure : GasPriceOpenFailure d wire reason) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ feltFailureAddr (GF := GF) reason ↦□ kind ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadGasPrice)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ FeltFailureError err v.pos' reason kind }} := by
  cases failure with
  | outer hn hu failure =>
    simpa only [feltFailureAddr, FeltFailureError] using
      wp_readGasPrice_outer_failure (GF := GF) c dst fields old v xs wire dq _ kind hn hu failure hp hbytes
  | @count d head payload tail argument enc major short =>
    simpa only [feltFailureAddr, skipFailureAddr, FeltFailureError, SkipFailureError] using
      wp_readGasPrice_count_failure (GF := GF) c dst fields old v xs payload tail head argument dq kind enc major short hp hbytes
  | @depth head payload tail argument enc major fit =>
    simpa only [feltFailureAddr, FeltFailureError, SkipFailureError] using
      wp_readGasPrice_depth_failure (GF := GF) c dst fields old v xs payload tail head argument dq kind enc major fit (by omega) hp hbytes

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPrice_open_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readGasPrice_open_failure
