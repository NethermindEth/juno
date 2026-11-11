import ProofCheck.InvokeFieldValid

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

/-- Errors preserve the actual bare-sentinel versus allocated-diagnostic
    distinction. Private failed map allocations are accounted for by the helper. -/
def InvokeFieldError (result : GoInterface) (pos : w64) : InvokeFieldFailure → IProp GF
  | .mode reason => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ DAModeError result pos kinds reason)
  | .felt reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ FeltFailureError err pos reason (feltFailureKind reason kinds))
  | .values reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ FeltValuesFailureError err pos reason kinds)
  | .resources reason => ResourceMapFailureOwned result pos reason
  | .scalar reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ ExpectedFailureError err pos (W8 0) reason (expectedFailureKind reason kinds))

theorem invokeFieldError_nonNil (result : GoInterface) (pos : w64) (reason : InvokeFieldFailure) :
    InvokeFieldError (GF := GF) result pos reason ⊢ ⌜result ≠ interface.nil⌝ := by
  cases reason with
  | mode reason =>
    isimp only [InvokeFieldError]
    iintro ⟨%kinds, _, Herr⟩
    iapply daModeError_nonNil result pos kinds reason
    iexact Herr
  | resources reason => exact resourceMapFailureOwned_nonNil result pos reason
  | felt | values | scalar =>
    isimp only [InvokeFieldError]
    iintro ⟨%err, %Heq, _⟩
    ipureintro; rw [Heq]; simp

/-- Unsigned parse failures consume zero. Overflow ignores the position because
    its actual error is bare; moving to the consumed boundary invents no offset. -/
theorem invokeDAModeError_boundary (result : GoInterface) (pos : w64) (kinds : FeltErrorKinds)
    (wire : List w8) (consumed : Nat) (reason : DAModeFailureReason)
    (failure : DAModeRejected wire consumed reason) :
    DAModeError (GF := GF) result pos kinds reason ⊢ DAModeError result (pos + W64 consumed) kinds reason := by
  cases failure with
  | expected =>
    iintro Herr
    isimp only [Int.natCast_zero, show pos + W64 0 = pos from by word]
    iexact Herr
  | overflow =>
    iintro Herr
    isimp only [DAModeError] at Herr
    isimp only [DAModeError]
    iexact Herr

theorem wp_invoke_field_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : InvokeValue) (field : InvokeField) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d consumed offset extraDepth : Nat) (reason : InvokeFieldFailure)
    (failure : InvokeFieldRejected d field wire consumed offset extraDepth reason)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       InvokeOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (invokeFieldReader field))) (Val #c)) (Val #(structFieldRef InvokeTransaction (invokeFieldName field) dst)))
    {{ (result : GoInterface), RET #result; InvokeOwned dst old ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ InvokeFieldError result (v.pos' + W64 offset) reason ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  iintro %Φ ⟨#Hinvokes, #Hschema, Howned, Hc, Hs⟩ HΦ
  isimp only [InvokeOwned] at Howned
  icases Howned with ⟨%h, Hdst, Hchildren⟩
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ Hschema
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  cases failure with
  | @hash wire offset why failure =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_hash_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readFelt_failure_bounded c (structFieldRef InvokeTransaction go!"TransactionHash" dst) h.TransactionHash' v xs wire dq d offset why (feltFailureKind why kinds) failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hkind Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.TransactionHash' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [InvokeFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @callData wire offset why failure =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_callData_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFeltValues_rejected c (structFieldRef InvokeTransaction go!"CallData" dst) h.CallData' v xs wire dq d offset why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.CallData' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [InvokeFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexact Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @signature wire offset why failure =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_signature_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFeltValues_rejected c (structFieldRef InvokeTransaction go!"TransactionSignature" dst) h.TransactionSignature' v xs wire dq d offset why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.TransactionSignature' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [InvokeFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexact Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @maxFee wire offset why failure =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_maxFee_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readFelt_failure_bounded c (structFieldRef InvokeTransaction go!"MaxFee" dst) h.MaxFee' v xs wire dq d offset why (feltFailureKind why kinds) failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hkind Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.MaxFee' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [InvokeFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @contract wire offset why failure =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_contract_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readFelt_failure_bounded c (structFieldRef InvokeTransaction go!"ContractAddress" dst) h.ContractAddress' v xs wire dq d offset why (feltFailureKind why kinds) failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hkind Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.ContractAddress' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [InvokeFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @version wire offset why failure =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_version_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readTransactionVersion_rejected c (structFieldRef InvokeTransaction go!"Version" dst) h.Version' v xs wire dq d offset why (feltFailureKind why kinds) failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hkind Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.Version' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [InvokeFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @entry wire offset why failure =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_entry_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readFelt_failure_bounded c (structFieldRef InvokeTransaction go!"EntryPointSelector" dst) h.EntryPointSelector' v xs wire dq d offset why (feltFailureKind why kinds) failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hkind Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.EntryPointSelector' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [InvokeFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @nonce wire offset why failure =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_nonce_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readFelt_failure_bounded c (structFieldRef InvokeTransaction go!"Nonce" dst) h.Nonce' v xs wire dq d offset why (feltFailureKind why kinds) failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hkind Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.Nonce' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [InvokeFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @sender wire offset why failure =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_sender_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readFelt_failure_bounded c (structFieldRef InvokeTransaction go!"SenderAddress" dst) h.SenderAddress' v xs wire dq d offset why (feltFailureKind why kinds) failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hkind Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.SenderAddress' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [InvokeFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @resources wire consumed offset extraDepth why failure =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_resources_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readResourceMap_rejected validator c (structFieldRef InvokeTransaction go!"ResourceBounds" dst) h.ResourceBounds' v xs wire dq d consumed offset extraDepth why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %result ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.ResourceBounds' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [InvokeFieldError]
      iexact Herr
    ipureintro; exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2.1, Hnext.2.2.2.1⟩
  | @tip wire why failure =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_tip_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 0) why) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind] at Hkind
    have hread := wp_readScalar_failure (GF := GF) false c (structFieldRef InvokeTransaction go!"Tip" dst) h.Tip' v xs wire dq why (expectedFailureKind why kinds) failure hp hbytes
    simp only [Bool.false_eq_true, ite_false] at hread
    wp_apply hread
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
    · iframe Hkind Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.Tip' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [InvokeFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨trivial, trivial, Int.le_refl _, hp.2⟩
  | @paymaster wire offset why failure =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_paymaster_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFeltValues_rejected c (structFieldRef InvokeTransaction go!"PaymasterData" dst) h.PaymasterData' v xs wire dq d offset why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.PaymasterData' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [InvokeFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexact Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @deployment wire offset why failure =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_deployment_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFeltValues_rejected c (structFieldRef InvokeTransaction go!"AccountDeploymentData" dst) h.AccountDeploymentData' v xs wire dq d offset why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.AccountDeploymentData' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [InvokeFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexact Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @nonceMode wire consumed why failure =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_nonceMode_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readDAMode_rejected c (structFieldRef InvokeTransaction go!"NonceDAMode" dst) h.NonceDAMode' v xs wire dq consumed why kinds failure hp hbytes
      $$ [Hfield Hc Hs] as %result ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hglobals Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.NonceDAMode' Hfield
    ihave Herr := invokeDAModeError_boundary result v.pos' kinds wire consumed why failure $$ Herr
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [InvokeFieldError]
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨Hnext.1, Hnext.1, Int.le_refl _, Hnext.2⟩

  | @feeMode wire consumed why failure =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_feeMode_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readDAMode_rejected c (structFieldRef InvokeTransaction go!"FeeDAMode" dst) h.FeeDAMode' v xs wire dq consumed why kinds failure hp hbytes
      $$ [Hfield Hc Hs] as %result ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hglobals Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.FeeDAMode' Hfield
    ihave Herr := invokeDAModeError_boundary result v.pos' kinds wire consumed why failure $$ Herr
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [InvokeFieldError]
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨Hnext.1, Hnext.1, Int.le_refl _, Hnext.2⟩

  | @proofFacts wire offset why failure =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_proofFacts_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readPlainFeltValues_rejected c (structFieldRef InvokeTransaction go!"ProofFacts" dst) h.ProofFacts' v xs wire dq d offset why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.ProofFacts' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [InvokeFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexact Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeFieldError_nonNil
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeDAModeError_boundary
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_invoke_field_rejected
