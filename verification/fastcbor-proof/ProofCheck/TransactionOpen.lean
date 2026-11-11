import ProofCheck.TransactionInitialize
import ProofCheck.StructOpenComplete
import ProofCheck.InitializationErrors

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

def transactionOpenName : TransactionSchema → GoString
  | .resourceBounds => internal.cbortransactions.OpenResourceBounds
  | .declareTransaction => internal.cbortransactions.OpenDeclareTransaction
  | .deployTransaction => internal.cbortransactions.OpenDeployTransaction
  | .invokeTransaction => internal.cbortransactions.OpenInvokeTransaction
  | .l1HandlerTransaction => internal.cbortransactions.OpenL1HandlerTransaction
  | .deployAccountTransaction => internal.cbortransactions.OpenDeployAccountTransaction
  | .transactionHashProjection => internal.cbortransactions.OpenTransactionHashProjection

/-- Every actual transaction opening wrapper obtains its own immutable table and
    canonical errors from initialized package state, then realizes the complete
    null/map/rejection contract for its definite-length map or null/undefined input. -/
theorem wp_openTransaction_complete (schema : TransactionSchema) (c : Loc) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (@! (transactionOpenName schema))) (Val #c))
    {{ (state : cursor.Struct) (result : GoInterface), RET (PairV #state #result);
       v.buf' ↦*{dq} xs ∗ ∃ (fields : Loc) (kinds : FeltErrorKinds),
       globalAddr (transactionGlobal schema) ↦□ fields ∗
       StaticFieldsShared fields (transactionKeys schema) (transactionNames schema) (transactionMask schema) ∗
       FeltErrorGlobals kinds ∗ StructOpenOutcome c fields v xs wire d kinds state result }} := by
  iintro %Φ ⟨#Hpkg, Hc, Hs⟩ HΦ
  ihave #Htable := transaction_initialized_table schema $$ Hpkg
  isimp only [TransactionTable] at Htable
  icases Htable with ⟨%fields, #Hglobal, #Hfields⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply cursor_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  cases schema <;> simp only [transactionOpenName]
  all_goals
    isimp only [transactionGlobal] at Hglobal
    wp_func_call
    wp_call
    wp_auto
    wp_apply wp_openStruct_complete c fields v xs wire dq d kinds hd depth hp hbytes
      $$ [Hc Hs] as %state %result ⟨Hs, Hout⟩
    · iframe Hglobals Hc Hs
    iapply HΦ
    iframe Hs
    iexists fields, kinds
    isimp only [transactionGlobal]
    iframe Hglobal Hfields Hglobals Hout

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_openTransaction_complete
