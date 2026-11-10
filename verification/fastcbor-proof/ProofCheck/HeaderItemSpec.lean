import ProofCheck.GasPriceSpec
import ProofCheck.HeaderValueFailureSpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Full successful Header field semantics, including arbitrary UTF-8 strings
    and complete nullable GasPrice records. Every constructor updates only its
    selected application field; unknown keys leave the Header unchanged. -/
inductive HeaderItemValid (d : Nat) : Option HeaderField → HeaderValue → List w8 → HeaderValue → Prop where
  | felt (field : HeaderFeltField) {old : HeaderValue} {wire : List w8} {value : Option (GoArray w64 4)}
      (valid : FeltValid d wire value) :
      HeaderItemValid d (some (headerFeltKey field)) old wire (headerSetFeltValue field old value)
  | scalar (field : HeaderScalarField) {old : HeaderValue} {wire : List w8} {value : w64}
      (valid : ScalarValid (headerScalarValue field old) wire value) :
      HeaderItemValid d (some (headerScalarKey field)) old wire (headerSetScalarValue field old value)
  | signatures {old : HeaderValue} {wire : List w8} {value : Option (List SignatureValue)}
      (valid : SignaturesValid d wire value) :
      HeaderItemValid d (some .signatures) old wire {old with signatures := value}
  | string {old : HeaderValue} {wire : List w8} {value : GoString}
      (valid : StringValid old.protocolVersion wire value) :
      HeaderItemValid d (some .protocolVersion) old wire {old with protocolVersion := value}
  | gas (second : Bool) {old : HeaderValue} {wire : List w8} {value : Option GasPriceValue}
      (valid : GasPointerValid d wire value) :
      HeaderItemValid d (some (headerGasKey second)) old wire (headerSetGasValue second old value)
  | ignored {old : HeaderValue} {wire : List w8} (valid : SkipValid d wire) :
      HeaderItemValid d none old wire old

/-- Complete field failures. Failed fields publish no replacement; a failed
    GasPrice retains the specified private partial value until caller cleanup.
    Consumption, diagnostic offset and retained child depth are separate. -/
inductive HeaderItemRejected (d : Nat) : Option HeaderField → List w8 → Nat → Nat → Nat → HeaderValueFailureReason → Prop where
  | felt (field : HeaderFeltField) {wire : List w8} {offset : Nat} {reason : FeltFailureReason}
      (failure : FeltFailure d wire offset reason) :
      HeaderItemRejected d (some (headerFeltKey field)) wire 0 offset 0 (.felt reason)
  | scalar (field : HeaderScalarField) {wire : List w8} {reason : ExpectedFailureReason}
      (failure : ScalarFailure wire reason) :
      HeaderItemRejected d (some (headerScalarKey field)) wire 0 0 0 (.felt (.expected (W8 0) reason))
  | signatures {wire : List w8} {consumed offset extraDepth : Nat} {reason : FeltFailureReason}
      (failure : SignaturesFailure d wire consumed offset extraDepth reason) :
      HeaderItemRejected d (some .signatures) wire consumed offset extraDepth (.felt reason)
  | string {wire : List w8} {reason : StringFailureReason} (failure : StringRejected wire reason) :
      HeaderItemRejected d (some .protocolVersion) wire 0 0 0 (.string reason)
  | gas (second : Bool) (retained : GasPriceValue) {wire : List w8} {consumed offset : Nat} {entered : Bool}
      {reason : GasPriceFailureReason}
      (failure : GasRecordRejected d ⟨none, none⟩ wire retained consumed offset entered reason) :
      HeaderItemRejected d (some (headerGasKey second)) wire consumed offset (if entered then 1 else 0) (.gas reason)
  | ignored {wire : List w8} {offset : Nat} {reason : SkipFailureReason} (failure : SkipFailure d wire offset reason) :
      HeaderItemRejected d none wire 0 offset 0 (.felt (.skipped reason))

/-- Every one of the fifteen schema identities has exactly the intended field
    category used by the typed decoder. This is an exhaustive enum case proof. -/
theorem headerField_category (field : HeaderField) :
    (∃ f, field = headerFeltKey f) ∨ (∃ f, field = headerScalarKey f) ∨
    field = .signatures ∨ field = .protocolVersion ∨ (∃ second, field = headerGasKey second) := by
  cases field with
  | hash => exact Or.inl ⟨.hash, rfl⟩
  | parentHash => exact Or.inl ⟨.parentHash, rfl⟩
  | globalStateRoot => exact Or.inl ⟨.globalStateRoot, rfl⟩
  | sequencerAddress => exact Or.inl ⟨.sequencerAddress, rfl⟩
  | gasETH => exact Or.inl ⟨.gasETH, rfl⟩
  | gasSTRK => exact Or.inl ⟨.gasSTRK, rfl⟩
  | number => exact Or.inr (Or.inl ⟨.number, rfl⟩)
  | transactionCount => exact Or.inr (Or.inl ⟨.transactions, rfl⟩)
  | eventCount => exact Or.inr (Or.inl ⟨.events, rfl⟩)
  | timestamp => exact Or.inr (Or.inl ⟨.timestamp, rfl⟩)
  | mode => exact Or.inr (Or.inl ⟨.mode, rfl⟩)
  | signatures => exact Or.inr (Or.inr (Or.inl rfl))
  | protocolVersion => exact Or.inr (Or.inr (Or.inr (Or.inl rfl)))
  | dataGas => exact Or.inr (Or.inr (Or.inr (Or.inr ⟨false, rfl⟩)))
  | l2Gas => exact Or.inr (Or.inr (Or.inr (Or.inr ⟨true, rfl⟩)))

/-- Every selected Header field has a specified outcome for arbitrary input
    and prior application values. No child-reader correctness WP is assumed. -/
theorem headerItem_classify (d : Nat) (choice : Option HeaderField) (old : HeaderValue)
    (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest value, wire = item ++ rest ∧ HeaderItemValid d choice old item value) ∨
    (∃ consumed offset extraDepth reason, HeaderItemRejected d choice wire consumed offset extraDepth reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨0, offset, 0, .felt (.skipped reason), .ignored failure⟩
  | some field =>
    rcases headerField_category field with ⟨f, rfl⟩ | ⟨f, rfl⟩ | rfl | rfl | ⟨second, rfl⟩
    · rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
      · exact Or.inl ⟨item, rest, headerSetFeltValue f old value, split, .felt f valid⟩
      · exact Or.inr ⟨0, offset, 0, .felt reason, .felt f failure⟩
    · rcases scalar_classify (headerScalarValue f old) wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
      · exact Or.inl ⟨item, rest, headerSetScalarValue f old value, split, .scalar f valid⟩
      · exact Or.inr ⟨0, 0, 0, .felt (.expected (W8 0) reason), .scalar f failure⟩
    · rcases signatures_classify d wire depth with ⟨item, rest, value, split, valid⟩ |
        ⟨consumed, offset, extraDepth, reason, failure⟩
      · exact Or.inl ⟨item, rest, {old with signatures := value}, split, .signatures valid⟩
      · exact Or.inr ⟨consumed, offset, extraDepth, .felt reason, .signatures failure⟩
    · rcases string_classify old.protocolVersion wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
      · exact Or.inl ⟨item, rest, {old with protocolVersion := value}, split, .string valid⟩
      · exact Or.inr ⟨0, 0, 0, .string reason, .string failure⟩
    · rcases gasPointer_classify d wire depth with ⟨item, rest, value, split, valid⟩ |
        ⟨retained, consumed, offset, entered, reason, failure⟩
      · exact Or.inl ⟨item, rest, headerSetGasValue second old value, split, .gas second valid⟩
      · exact Or.inr ⟨consumed, offset, if entered then 1 else 0, .gas reason, .gas second retained failure⟩

theorem headerItem_nonempty (d : Nat) (choice : Option HeaderField) (old value : HeaderValue)
    (wire : List w8) (valid : HeaderItemValid d choice old wire value) : 0 < wire.length := by
  cases valid with
  | felt field valid => cases valid <;> simp only [List.length_append, List.length_cons, List.length_nil] <;> omega
  | scalar field valid => cases valid <;> simp
  | signatures valid => cases valid <;> simp
  | string valid => exact stringValid_nonempty _ _ _ valid
  | gas second valid => exact gasPointer_nonempty _ _ _ valid
  | ignored valid => exact skipValid_nonempty _ _ valid

theorem headerItemRejected_bounds (d consumed offset extraDepth : Nat) (choice : Option HeaderField)
    (wire : List w8) (reason : HeaderValueFailureReason) (depth : d ≤ 32)
    (failure : HeaderItemRejected d choice wire consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length ∧ d + extraDepth ≤ 32 := by
  cases failure with
  | felt field failure =>
    have bound := feltFailure_offset _ _ _ _ failure
    simp only [Nat.add_zero]; omega
  | scalar field failure => simp; omega
  | signatures failure =>
    have bound := signaturesFailure_bounds _ _ _ _ _ _ failure
    exact ⟨bound.1, bound.2, signaturesFailure_depth _ _ _ _ _ _ depth failure⟩
  | string failure => simp; omega
  | gas second retained failure => exact gasRecordRejected_bounds _ _ _ _ _ _ _ _ failure depth
  | ignored failure =>
    have bound := skipFailure_offset _ _ _ _ failure
    simp only [Nat.add_zero]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerField_category
#print axioms Perennial.github_com.NethermindEth.juno.core.headerItem_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.headerItem_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.headerItemRejected_bounds
