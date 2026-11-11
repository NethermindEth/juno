import ProofCheck.DeclareFieldValid

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
def DeclareFieldError (result : GoInterface) (pos : w64) : DeclareFieldFailure → IProp GF
  | .mode reason => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ DAModeError result pos kinds reason)
  | .felt reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ FeltFailureError err pos reason (feltFailureKind reason kinds))
  | .values reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ FeltValuesFailureError err pos reason kinds)
  | .resources reason => ResourceMapFailureOwned result pos reason
  | .scalar reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ ExpectedFailureError err pos (W8 0) reason (expectedFailureKind reason kinds))

theorem declareFieldError_nonNil (result : GoInterface) (pos : w64) (reason : DeclareFieldFailure) :
    DeclareFieldError (GF := GF) result pos reason ⊢ ⌜result ≠ interface.nil⌝ := by
  cases reason with
  | mode reason =>
    isimp only [DeclareFieldError]
    iintro ⟨%kinds, _, Herr⟩
    iapply daModeError_nonNil result pos kinds reason
    iexact Herr
  | resources reason => exact resourceMapFailureOwned_nonNil result pos reason
  | felt | values | scalar =>
    isimp only [DeclareFieldError]
    iintro ⟨%err, %Heq, _⟩
    ipureintro; rw [Heq]; simp

/-- Unsigned parse failures consume zero. Overflow ignores the position because
    its actual error is bare; moving to the consumed boundary invents no offset. -/
theorem daModeError_boundary (result : GoInterface) (pos : w64) (kinds : FeltErrorKinds)
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

theorem wp_declare_field_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : DeclareValue) (field : DeclareField) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d consumed offset extraDepth : Nat) (reason : DeclareFieldFailure)
    (failure : DeclareFieldRejected d field wire consumed offset extraDepth reason)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       DeclareOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (declareFieldReader field))) (Val #c)) (Val #(structFieldRef DeclareTransaction (declareFieldName field) dst)))
    {{ (result : GoInterface), RET #result; DeclareOwned dst old ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ DeclareFieldError result (v.pos' + W64 offset) reason ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  iintro %Φ ⟨#Hdeclares, #Hschema, Howned, Hc, Hs⟩ HΦ
  isimp only [DeclareOwned] at Howned
  icases Howned with ⟨%h, Hdst, Hchildren⟩
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ Hschema
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  cases failure with
  | @hash wire offset why failure =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_hash_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readFelt_failure_bounded c (structFieldRef DeclareTransaction go!"TransactionHash" dst) h.TransactionHash' v xs wire dq d offset why (feltFailureKind why kinds) failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hkind Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.TransactionHash' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [DeclareFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @classHash wire offset why failure =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_classHash_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readFelt_failure_bounded c (structFieldRef DeclareTransaction go!"ClassHash" dst) h.ClassHash' v xs wire dq d offset why (feltFailureKind why kinds) failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hkind Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.ClassHash' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [DeclareFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @sender wire offset why failure =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_sender_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readFelt_failure_bounded c (structFieldRef DeclareTransaction go!"SenderAddress" dst) h.SenderAddress' v xs wire dq d offset why (feltFailureKind why kinds) failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hkind Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.SenderAddress' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [DeclareFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @maxFee wire offset why failure =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_maxFee_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readFelt_failure_bounded c (structFieldRef DeclareTransaction go!"MaxFee" dst) h.MaxFee' v xs wire dq d offset why (feltFailureKind why kinds) failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hkind Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.MaxFee' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [DeclareFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @signature wire offset why failure =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_signature_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFeltValues_rejected c (structFieldRef DeclareTransaction go!"TransactionSignature" dst) h.TransactionSignature' v xs wire dq d offset why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.TransactionSignature' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [DeclareFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexact Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @nonce wire offset why failure =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_nonce_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readFelt_failure_bounded c (structFieldRef DeclareTransaction go!"Nonce" dst) h.Nonce' v xs wire dq d offset why (feltFailureKind why kinds) failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hkind Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.Nonce' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [DeclareFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @version wire offset why failure =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_version_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readTransactionVersion_rejected c (structFieldRef DeclareTransaction go!"Version" dst) h.Version' v xs wire dq d offset why (feltFailureKind why kinds) failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hkind Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.Version' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [DeclareFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @compiledClassHash wire offset why failure =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_compiledClassHash_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readFelt_failure_bounded c (structFieldRef DeclareTransaction go!"CompiledClassHash" dst) h.CompiledClassHash' v xs wire dq d offset why (feltFailureKind why kinds) failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hkind Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.CompiledClassHash' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [DeclareFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @resources wire consumed offset extraDepth why failure =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_resources_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readResourceMap_rejected validator c (structFieldRef DeclareTransaction go!"ResourceBounds" dst) h.ResourceBounds' v xs wire dq d consumed offset extraDepth why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %result ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.ResourceBounds' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [DeclareFieldError]
      iexact Herr
    ipureintro; exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2.1, Hnext.2.2.2.1⟩
  | @tip wire why failure =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_tip_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 0) why) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind] at Hkind
    have hread := wp_readScalar_failure (GF := GF) false c (structFieldRef DeclareTransaction go!"Tip" dst) h.Tip' v xs wire dq why (expectedFailureKind why kinds) failure hp hbytes
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
    · unfold DeclareOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [DeclareFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨trivial, trivial, Int.le_refl _, hp.2⟩
  | @paymaster wire offset why failure =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_paymaster_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFeltValues_rejected c (structFieldRef DeclareTransaction go!"PaymasterData" dst) h.PaymasterData' v xs wire dq d offset why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.PaymasterData' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [DeclareFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexact Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @deployment wire offset why failure =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_deployment_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFeltValues_rejected c (structFieldRef DeclareTransaction go!"AccountDeploymentData" dst) h.AccountDeploymentData' v xs wire dq d offset why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.AccountDeploymentData' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [DeclareFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexact Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @nonceMode wire consumed why failure =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_nonceMode_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readDAMode_rejected c (structFieldRef DeclareTransaction go!"NonceDAMode" dst) h.NonceDAMode' v xs wire dq consumed why kinds failure hp hbytes
      $$ [Hfield Hc Hs] as %result ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hglobals Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.NonceDAMode' Hfield
    ihave Herr := daModeError_boundary result v.pos' kinds wire consumed why failure $$ Herr
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [DeclareFieldError]
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨Hnext.1, Hnext.1, Int.le_refl _, Hnext.2⟩

  | @feeMode wire consumed why failure =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_feeMode_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readDAMode_rejected c (structFieldRef DeclareTransaction go!"FeeDAMode" dst) h.FeeDAMode' v xs wire dq consumed why kinds failure hp hbytes
      $$ [Hfield Hc Hs] as %result ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hglobals Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.FeeDAMode' Hfield
    ihave Herr := daModeError_boundary result v.pos' kinds wire consumed why failure $$ Herr
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [DeclareFieldError]
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨Hnext.1, Hnext.1, Int.le_refl _, Hnext.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.declareFieldError_nonNil
#print axioms Perennial.github_com.NethermindEth.juno.core.daModeError_boundary
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_declare_field_rejected
