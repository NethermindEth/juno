import ProofCheck.TransactionOwnership

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

/-- The actual dispatcher realizes each independently specified outcome. A
    failed payload remains private and never replaces the caller's interface. -/
theorem wp_readTransaction_spec (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : GoInterface) (tags : CBORTransactionTags) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (raw : TransactionDispatchResult)
    (distinct : TransactionTagsDistinct tags) (spec : TransactionDispatchSpec d tags wire raw)
    (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (Val (@! cborReadTransaction)) (Val #c)) (Val #dst)) (Val #tags))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ TransactionReadOwned c dst old v xs wire result (transactionResult raw) }} := by
  wp_start_folded as ⟨Hdst, Hc, Hs⟩
  cases spec with
  | nullish head rest hn =>
    wp_apply wp_transaction_nullish c dst old tags v xs rest head dq hn hp hbytes
      $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs, %Hstep⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    isimp only [transactionResult, TransactionReadOwned]
    iexists interface.nil, (1 : Nat)
    isimp only [TransactionValueOwned]
    iframe Hdst Hc
    ipureintro
    exact ⟨trivial, by simp; omega, Hstep⟩
  | @tagFailure wire reason hn hu failure =>
    wp_apply wp_transaction_tag_rejected c dst old tags v xs wire dq reason hn hu failure hp hbytes
      $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, %kinds, Hglobals, Herr⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    isimp only [transactionResult, TransactionReadOwned]
    have hzero : v.pos' + W64 (0 : Nat) = v.pos' := by word
    have hdzero : v.depth' + W64 (0 : Nat) = v.depth' := by word
    isimp only [hzero, hdzero]
    iframe Hdst Hc
    isplitl [Hglobals Herr]
    · isimp only [TransactionErrorOwned]
      iexists err, kinds
      iframe Hglobals Herr
      ipureintro; rfl
    ipureintro
    exact ⟨by simp, by simp, by simp, by omega, hp.2⟩
  | @unknown head payload rest tag enc major unknown =>
    wp_apply wp_transaction_unknown_tag c dst old tags v xs payload rest head tag dq enc major unknown hp hbytes
      $$ [Hdst Hc Hs] as %result ⟨Hdst, Hc, Hs, Herror, %Hstep⟩
    · iframe Hdst Hc Hs
    icases Herror with ⟨%kinds, Hglobals, %Hresult⟩
    iapply HΦ
    iframe Hs
    isimp only [transactionResult, TransactionReadOwned]
    have hdzero : v.depth' + W64 (0 : Nat) = v.depth' := by word
    isimp only [hdzero, Int.natCast_add, Int.natCast_one]
    iframe Hdst Hc
    isplitl [Hglobals]
    · isimp only [TransactionErrorOwned]
      iexists kinds
      iframe Hglobals
      ipureintro; exact Hresult
    ipureintro
    have hstep := Hstep.1
    simp only [Int.natCast_add, Int.natCast_one, Int.add_assoc] at hstep ⊢
    exact ⟨Hresult.2, hstep, hstep, by omega, Hstep.2.1⟩
  | @payload head payload body value enc major valid =>
    cases value with
    | declare value =>
      cases value with
      | success value rest =>
        obtain ⟨item, rfl, valid⟩ := valid
        wp_apply wp_transaction_declare_valid validator c dst old tags v xs payload item rest head value dq d
          distinct enc major valid hd hp (by simpa only [List.append_assoc] using hbytes)
          $$ [Hdst Hc Hs] as %out ⟨Hdst, Howned, Hc, Hs, %Hstep⟩
        · iframe Hdst Hc Hs
        iapply HΦ
        iframe Hs
        isimp only [transactionResult, TransactionReadOwned]
        iexists interface.mkOk (go.PointerType DeclareTransaction.ty) #out, (1 + payload.length + item.length)
        isimp only [Int.natCast_add, Int.natCast_one]
        iframe Hdst Hc
        isplitl [Howned]
        · isimp only [TransactionValueOwned]
          iexists out
          iframe Howned
          ipureintro; rfl
        ipureintro
        refine ⟨trivial, by simp; omega, ?_, Hstep.2⟩
        simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.1
      | failure retained consumed offset dep reason =>
        wp_apply wp_transaction_declare_rejected validator c dst old tags v xs payload body head retained dq
          d consumed offset dep reason distinct enc major valid hd hp hbytes
          $$ [Hdst Hc Hs] as %result ⟨Hdst, Howned, Hc, Hs, Herr, %Hstep⟩
        · iframe Hdst Hc Hs
        ihave %HnonNil := declareFailureOwned_nonNil result _ reason $$ Herr
        iapply HΦ
        iframe Hs
        isimp only [transactionResult, TransactionReadOwned]
        isimp only [Int.natCast_add, Int.natCast_one]
        iframe Hdst Hc
        isplitl [Howned Herr]
        · isimp only [TransactionErrorOwned]
          iframe Howned
          iexact Herr
        ipureintro
        refine ⟨HnonNil, ?_, ?_, Hstep.2.2⟩
        · simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.1
        · simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.2.1
    | deploy value =>
      cases value with
      | success value rest =>
        obtain ⟨item, rfl, valid⟩ := valid
        wp_apply wp_transaction_deploy_valid validator c dst old tags v xs payload item rest head value dq d
          distinct enc major valid hd hp (by simpa only [List.append_assoc] using hbytes)
          $$ [Hdst Hc Hs] as %out ⟨Hdst, Howned, Hc, Hs, %Hstep⟩
        · iframe Hdst Hc Hs
        iapply HΦ
        iframe Hs
        isimp only [transactionResult, TransactionReadOwned]
        iexists interface.mkOk (go.PointerType DeployTransaction.ty) #out, (1 + payload.length + item.length)
        isimp only [Int.natCast_add, Int.natCast_one]
        iframe Hdst Hc
        isplitl [Howned]
        · isimp only [TransactionValueOwned]
          iexists out
          iframe Howned
          ipureintro; rfl
        ipureintro
        refine ⟨trivial, by simp; omega, ?_, Hstep.2⟩
        simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.1
      | failure retained consumed offset dep reason =>
        wp_apply wp_transaction_deploy_rejected validator c dst old tags v xs payload body head retained dq
          d consumed offset dep reason distinct enc major valid hd hp hbytes
          $$ [Hdst Hc Hs] as %err ⟨Hdst, Howned, Hc, Hs, Herr, %Hstep⟩
        · iframe Hdst Hc Hs
        have hdep : (if dep then v.depth' + W64 1 else v.depth') = v.depth' + W64 (if dep then 1 else 0 : Nat) := by
          cases dep <;> simp <;> word
        isimp only [hdep] at Hc
        iapply HΦ
        iframe Hs
        isimp only [transactionResult, TransactionReadOwned]
        isimp only [Int.natCast_add, Int.natCast_one]
        iframe Hdst Hc
        isplitl [Howned Herr]
        · isimp only [TransactionErrorOwned]
          iframe Howned
          iexists err
          iframe Herr
          ipureintro; rfl
        ipureintro
        refine ⟨by simp, ?_, ?_, Hstep.2.2⟩
        · simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.1
        · simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.2.1
    | invoke value =>
      cases value with
      | success value rest =>
        obtain ⟨item, rfl, valid⟩ := valid
        wp_apply wp_transaction_invoke_valid validator c dst old tags v xs payload item rest head value dq d
          distinct enc major valid hd hp (by simpa only [List.append_assoc] using hbytes)
          $$ [Hdst Hc Hs] as %out ⟨Hdst, Howned, Hc, Hs, %Hstep⟩
        · iframe Hdst Hc Hs
        iapply HΦ
        iframe Hs
        isimp only [transactionResult, TransactionReadOwned]
        iexists interface.mkOk (go.PointerType InvokeTransaction.ty) #out, (1 + payload.length + item.length)
        isimp only [Int.natCast_add, Int.natCast_one]
        iframe Hdst Hc
        isplitl [Howned]
        · isimp only [TransactionValueOwned]
          iexists out
          iframe Howned
          ipureintro; rfl
        ipureintro
        refine ⟨trivial, by simp; omega, ?_, Hstep.2⟩
        simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.1
      | failure retained consumed offset dep reason =>
        wp_apply wp_transaction_invoke_rejected validator c dst old tags v xs payload body head retained dq
          d consumed offset dep reason distinct enc major valid hd hp hbytes
          $$ [Hdst Hc Hs] as %result ⟨Hdst, Howned, Hc, Hs, Herr, %Hstep⟩
        · iframe Hdst Hc Hs
        ihave %HnonNil := invokeFailureOwned_nonNil result _ reason $$ Herr
        iapply HΦ
        iframe Hs
        isimp only [transactionResult, TransactionReadOwned]
        isimp only [Int.natCast_add, Int.natCast_one]
        iframe Hdst Hc
        isplitl [Howned Herr]
        · isimp only [TransactionErrorOwned]
          iframe Howned
          iexact Herr
        ipureintro
        refine ⟨HnonNil, ?_, ?_, Hstep.2.2⟩
        · simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.1
        · simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.2.1
    | l1Handler value =>
      cases value with
      | success value rest =>
        obtain ⟨item, rfl, valid⟩ := valid
        wp_apply wp_transaction_l1Handler_valid validator c dst old tags v xs payload item rest head value dq d
          distinct enc major valid hd hp (by simpa only [List.append_assoc] using hbytes)
          $$ [Hdst Hc Hs] as %out ⟨Hdst, Howned, Hc, Hs, %Hstep⟩
        · iframe Hdst Hc Hs
        iapply HΦ
        iframe Hs
        isimp only [transactionResult, TransactionReadOwned]
        iexists interface.mkOk (go.PointerType L1HandlerTransaction.ty) #out, (1 + payload.length + item.length)
        isimp only [Int.natCast_add, Int.natCast_one]
        iframe Hdst Hc
        isplitl [Howned]
        · isimp only [TransactionValueOwned]
          iexists out
          iframe Howned
          ipureintro; rfl
        ipureintro
        refine ⟨trivial, by simp; omega, ?_, Hstep.2⟩
        simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.1
      | failure retained consumed offset dep reason =>
        wp_apply wp_transaction_l1Handler_rejected validator c dst old tags v xs payload body head retained dq
          d consumed offset dep reason distinct enc major valid hd hp hbytes
          $$ [Hdst Hc Hs] as %err ⟨Hdst, Howned, Hc, Hs, Herr, %Hstep⟩
        · iframe Hdst Hc Hs
        have hdep : (if dep then v.depth' + W64 1 else v.depth') = v.depth' + W64 (if dep then 1 else 0 : Nat) := by
          cases dep <;> simp <;> word
        isimp only [hdep] at Hc
        iapply HΦ
        iframe Hs
        isimp only [transactionResult, TransactionReadOwned]
        isimp only [Int.natCast_add, Int.natCast_one]
        iframe Hdst Hc
        isplitl [Howned Herr]
        · isimp only [TransactionErrorOwned]
          iframe Howned
          iexists err
          iframe Herr
          ipureintro; rfl
        ipureintro
        refine ⟨by simp, ?_, ?_, Hstep.2.2⟩
        · simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.1
        · simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.2.1
    | deployAccount value =>
      cases value with
      | success value rest =>
        obtain ⟨item, rfl, valid⟩ := valid
        wp_apply wp_transaction_deployAccount_valid validator c dst old tags v xs payload item rest head value dq d
          distinct enc major valid hd hp (by simpa only [List.append_assoc] using hbytes)
          $$ [Hdst Hc Hs] as %out ⟨Hdst, Howned, Hc, Hs, %Hstep⟩
        · iframe Hdst Hc Hs
        iapply HΦ
        iframe Hs
        isimp only [transactionResult, TransactionReadOwned]
        iexists interface.mkOk (go.PointerType DeployAccountTransaction.ty) #out, (1 + payload.length + item.length)
        isimp only [Int.natCast_add, Int.natCast_one]
        iframe Hdst Hc
        isplitl [Howned]
        · isimp only [TransactionValueOwned]
          iexists out
          iframe Howned
          ipureintro; rfl
        ipureintro
        refine ⟨trivial, by simp; omega, ?_, Hstep.2⟩
        simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.1
      | failure retained consumed offset dep reason =>
        wp_apply wp_transaction_deployAccount_rejected validator c dst old tags v xs payload body head retained dq
          d consumed offset dep reason distinct enc major valid hd hp hbytes
          $$ [Hdst Hc Hs] as %result ⟨Hdst, Howned, Hc, Hs, Herr, %Hstep⟩
        · iframe Hdst Hc Hs
        ihave %HnonNil := deployAccountFailureOwned_nonNil result _ reason $$ Herr
        iapply HΦ
        iframe Hs
        isimp only [transactionResult, TransactionReadOwned]
        isimp only [Int.natCast_add, Int.natCast_one]
        iframe Hdst Hc
        isplitl [Howned Herr]
        · isimp only [TransactionErrorOwned]
          iframe Howned
          iexact Herr
        ipureintro
        refine ⟨HnonNil, ?_, ?_, Hstep.2.2⟩
        · simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.1
        · simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.2.1

/-- Complete all-input contract for the production transaction reader, with an
    independently unique value or failure and exact input/cursor ownership. -/
theorem wp_readTransaction_complete (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : GoInterface) (tags : CBORTransactionTags) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (distinct : TransactionTagsDistinct tags)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (Val (@! cborReadTransaction)) (Val #c)) (Val #dst)) (Val #tags))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ outcome : TransactionResult,
       ⌜TransactionSpec d tags wire outcome ∧
         ∀ other, TransactionSpec d tags wire other → other = outcome⌝ ∗
       TransactionReadOwned c dst old v xs wire result outcome }} := by
  wp_start_folded as ⟨Hdst, Hc, Hs⟩
  obtain ⟨raw, spec⟩ := transactionDispatch_classify d tags wire hdepth
  wp_apply wp_readTransaction_spec validator c dst old tags v xs wire dq d raw distinct spec hd hp hbytes
    $$ [Hdst Hc Hs] as %result ⟨Hs, Howned⟩
  · iframe Hdst Hc Hs
  iapply HΦ
  iframe Hs
  iexists transactionResult raw
  iframe Howned
  ipureintro
  refine ⟨⟨raw, spec, rfl⟩, ?_⟩
  intro other otherSpec
  exact transactionSpec_unique d tags wire distinct other _ otherSpec ⟨raw, spec, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readTransaction_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readTransaction_complete
