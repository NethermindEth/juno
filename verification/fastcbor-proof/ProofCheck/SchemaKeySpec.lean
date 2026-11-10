import ProofCheck.KeySpec
import ProofCheck.HeaderKeys
import ProofCheck.GasPriceKeys

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Schema interpretation follows complete key decoding. Integer keys are
    unknown; text keys are interpreted by the independent schema-name relation. -/
def SchemaKeyValid {Field : Type} (meaning : GoString → Option Field → Prop)
    (wire : List w8) (choice : Option Field) : Prop :=
  ∃ value, KeyValid wire value ∧ match value with
    | none => choice = none
    | some key => meaning key choice

/-- Total text-name interpretation lifts the all-input key classification to a
    schema. Unknown names remain successful keys whose values must be skipped. -/
theorem schemaKey_classify {Field : Type} (meaning : GoString → Option Field → Prop)
    (total : ∀ key, ∃ choice, meaning key choice) (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ SchemaKeyValid meaning item choice) ∨
    (∃ reason, KeyRejected wire reason) := by
  rcases key_classify wire with ⟨item, rest, value, split, valid⟩ | failure
  · cases value with
    | none => exact Or.inl ⟨item, rest, none, split, none, valid, rfl⟩
    | some key =>
      obtain ⟨choice, hm⟩ := total key
      exact Or.inl ⟨item, rest, choice, split, some key, valid, hm⟩
  · exact Or.inr failure

/-- Unique text-name interpretation gives exact schema field identity as well
    as the unique consumed prefix and unread suffix. -/
theorem schemaKey_prefix_unique {Field : Type} (meaning : GoString → Option Field → Prop)
    (unique : ∀ key a b, meaning key a → meaning key b → a = b)
    (left right restLeft restRight : List w8) (a b : Option Field)
    (first : SchemaKeyValid meaning left a) (second : SchemaKeyValid meaning right b)
    (same : left ++ restLeft = right ++ restRight) :
    left = right ∧ a = b ∧ restLeft = restRight := by
  obtain ⟨x, hx, mx⟩ := first
  obtain ⟨y, hy, my⟩ := second
  obtain ⟨rfl, rfl, rfl⟩ := keyValid_prefix_unique left right restLeft restRight x y hx hy same
  refine ⟨rfl, ?_, rfl⟩
  cases x with
  | none => exact mx.trans my.symm
  | some key => exact unique key a b mx my

theorem schemaKey_nonempty {Field : Type} (meaning : GoString → Option Field → Prop)
    (wire : List w8) (choice : Option Field) (valid : SchemaKeyValid meaning wire choice) :
    0 < wire.length := by
  obtain ⟨value, valid, _⟩ := valid
  exact keyValid_nonempty wire value valid

theorem schemaKey_not_rejected {Field : Type} (meaning : GoString → Option Field → Prop)
    (item rest : List w8) (choice : Option Field) (reason : KeyFailureReason)
    (valid : SchemaKeyValid meaning item choice) : ¬ KeyRejected (item ++ rest) reason := by
  obtain ⟨value, valid, _⟩ := valid
  exact keyValid_not_rejected item rest value reason valid

/-- All Header identities in their application-schema order, including the two
    legacy gas-price wire names. This list is checked against the actual metadata. -/
def headerSchemaFields : List HeaderField :=
  [.hash, .parentHash, .number, .globalStateRoot, .sequencerAddress,
   .transactionCount, .eventCount, .timestamp, .protocolVersion, .gasETH,
   .signatures, .gasSTRK, .mode, .dataGas, .l2Gas]

theorem headerSchema_names : headerFieldNames = headerSchemaFields.map headerFieldWireName := rfl

theorem headerSchema_member (field : HeaderField) : headerFieldWireName field ∈ headerFieldNames :=
  List.mem_of_getElem? (headerKey_lookup field)

theorem headerSchema_fold_injective (a b : HeaderField)
    (same : (headerFieldWireName a).map fieldFoldByte = (headerFieldWireName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem headerKeyMeaning_total (key : GoString) : ∃ choice, HeaderKeyTextMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (headerFieldWireName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · refine ⟨none, ?_⟩
    intro name member same
    rw [headerSchema_names] at member
    obtain ⟨field, _, rfl⟩ := List.mem_map.mp member
    exact known ⟨field, same⟩

theorem headerKeyMeaning_unique (key : GoString) (a b : Option HeaderField)
    (first : HeaderKeyTextMeaning key a) (second : HeaderKeyTextMeaning key b) : a = b := by
  cases a with
  | none =>
    cases b with
    | none => rfl
    | some field => exact False.elim (first _ (headerSchema_member field) second)
  | some field =>
    cases b with
    | none => exact False.elim (second _ (headerSchema_member field) first)
    | some other => exact congrArg some (headerSchema_fold_injective field other (first.symm.trans second))

theorem gasSchema_member (wei : Bool) : gasPriceFieldName wei ∈ gasFieldNames :=
  List.mem_of_getElem? (gasKey_lookup wei)

theorem gasSchema_fold_injective (a b : Bool)
    (same : (gasPriceFieldName a).map fieldFoldByte = (gasPriceFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem gasKeyMeaning_total (key : GoString) : ∃ choice, GasKeyTextMeaning key choice := by
  classical
  by_cases known : ∃ wei, key.map fieldFoldByte = (gasPriceFieldName wei).map fieldFoldByte
  · obtain ⟨wei, found⟩ := known
    exact ⟨some wei, found⟩
  · refine ⟨none, ?_⟩
    intro name member same
    have names : gasFieldNames = [gasPriceFieldName true, gasPriceFieldName false] := rfl
    rw [names] at member
    simp only [List.mem_cons, List.not_mem_nil, or_false] at member
    rcases member with rfl | rfl
    · exact known ⟨true, same⟩
    · exact known ⟨false, same⟩

theorem gasKeyMeaning_unique (key : GoString) (a b : Option Bool)
    (first : GasKeyTextMeaning key a) (second : GasKeyTextMeaning key b) : a = b := by
  cases a with
  | none =>
    cases b with
    | none => rfl
    | some wei => exact False.elim (first _ (gasSchema_member wei) second)
  | some wei =>
    cases b with
    | none => exact False.elim (second _ (gasSchema_member wei) first)
    | some other => exact congrArg some (gasSchema_fold_injective wei other (first.symm.trans second))

/-- Full Header/GasPrice key specifications, without the short-ASCII restriction
    of the existing production-proved Supported relations. -/
abbrev HeaderKeyValid := SchemaKeyValid HeaderKeyTextMeaning
abbrev GasKeyValid := SchemaKeyValid GasKeyTextMeaning

theorem headerKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ HeaderKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify HeaderKeyTextMeaning headerKeyMeaning_total wire

theorem gasKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ GasKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify GasKeyTextMeaning gasKeyMeaning_total wire

/-- ASCII case folding never turns a non-ASCII byte into an ASCII byte. -/
theorem fieldFoldByte_ascii (b : w8) : uint.Z (fieldFoldByte b) < 128 ↔ uint.Z b < 128 := by
  rw [fieldFoldByte_value]
  split <;> omega

theorem fieldFold_preserves_ascii (key : GoString) :
    ASCIIBytes (key.map fieldFoldByte) ↔ ASCIIBytes key := by
  constructor
  · intro ascii b member
    exact (fieldFoldByte_ascii b).mp (ascii _ (List.mem_map.mpr ⟨b, member, rfl⟩))
  · intro ascii b member
    obtain ⟨original, originalMember, rfl⟩ := List.mem_map.mp member
    exact (fieldFoldByte_ascii original).mpr (ascii _ originalMember)

/-- A valid non-ASCII key is an unknown Header name, rather than a spelling of
    any known field. Its UTF-8 validity is checked separately by KeyValid. -/
theorem headerKeyMeaning_nonascii (key : GoString) (nonascii : ¬ ASCIIBytes key) :
    HeaderKeyTextMeaning key none := by
  intro name member same
  apply nonascii
  apply (fieldFold_preserves_ascii key).mp
  rw [same]
  exact (fieldFold_preserves_ascii name).mpr (headerFieldNames_ascii name member)

theorem gasKeyMeaning_nonascii (key : GoString) (nonascii : ¬ ASCIIBytes key) :
    GasKeyTextMeaning key none := by
  intro name member same
  apply nonascii
  apply (fieldFold_preserves_ascii key).mp
  rw [same]
  exact (fieldFold_preserves_ascii name).mpr (gasFieldNames_ascii name member)

/-- The current production-proved Header key family is included in the full
    contract; accepting additional text widths/Unicode does not change its meaning. -/
theorem headerKeySupported_valid (wire : List w8) (choice : Option HeaderField)
    (bound : wire.length < 2^64) (supported : HeaderKeySupported wire choice) :
    HeaderKeyValid wire choice := by
  cases supported with
  | text enc major width ascii meaning =>
    refine ⟨some _, .text enc major ?_ (utf8Valid_ascii _ ascii), meaning⟩
    simp only [List.length_append, List.length_cons] at bound
    word
  | integer enc major range => exact ⟨none, .integer enc major range, rfl⟩

theorem gasKeySupported_valid (wire : List w8) (choice : Option Bool)
    (bound : wire.length < 2^64) (supported : GasKeySupported wire choice) :
    GasKeyValid wire choice := by
  cases supported with
  | text enc major width ascii meaning =>
    refine ⟨some _, .text enc major ?_ (utf8Valid_ascii _ ascii), meaning⟩
    simp only [List.length_append, List.length_cons] at bound
    word
  | integer enc major range => exact ⟨none, .integer enc major range, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.schemaKey_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.schemaKey_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.schemaKey_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.schemaKey_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.headerSchema_names
#print axioms Perennial.github_com.NethermindEth.juno.core.headerSchema_member
#print axioms Perennial.github_com.NethermindEth.juno.core.headerSchema_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.headerKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.headerKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.gasSchema_member
#print axioms Perennial.github_com.NethermindEth.juno.core.gasSchema_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.gasKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.gasKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.headerKey_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.gasKey_classify

#print axioms Perennial.github_com.NethermindEth.juno.core.fieldFoldByte_ascii

#print axioms Perennial.github_com.NethermindEth.juno.core.fieldFold_preserves_ascii

#print axioms Perennial.github_com.NethermindEth.juno.core.headerKeyMeaning_nonascii

#print axioms Perennial.github_com.NethermindEth.juno.core.gasKeyMeaning_nonascii

#print axioms Perennial.github_com.NethermindEth.juno.core.headerKeySupported_valid

#print axioms Perennial.github_com.NethermindEth.juno.core.gasKeySupported_valid
