import ProofCheck.HeaderAPISpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Relate the actual GasPrice seen mask to semantic field membership. -/
def GasSeen (mask : w64) (fields : List Bool) : Prop :=
  ∀ wei, GasKeyFresh mask (some wei) ↔ wei ∉ fields

theorem gasKeyMasks_disjoint (a b : Bool) :
    ((W64 1 <<< gasKeyIndex a) &&& (W64 1 <<< gasKeyIndex b) = W64 0) ↔ a ≠ b := by
  cases a <;> cases b <;> decide

theorem gasKeyFresh_mark_iff (mask : w64) (choice : Option Bool) (wei : Bool) :
    GasKeyFresh (gasKeySeen mask choice) (some wei) ↔
      GasKeyFresh mask (some wei) ∧ choice ≠ some wei := by
  cases choice with
  | none => simp [gasKeySeen, GasKeyFresh]
  | some found =>
    have hor (a b : w64) : a ||| b = W64 0 ↔ a = W64 0 ∧ b = W64 0 := BitVec.or_eq_zero_iff
    simp only [gasKeySeen, GasKeyFresh, BitVec.and_or_distrib_right, hor,
      gasKeyMasks_disjoint, ne_eq, Option.some.injEq]

theorem gasSeen_zero : GasSeen (W64 0) [] := by
  intro wei
  simp [GasKeyFresh, BitVec.zero_and]

theorem gasSeen_mark (mask : w64) (fields : List Bool) (choice : Option Bool) (seen : GasSeen mask fields) :
    GasSeen (gasKeySeen mask choice) (gasChoiceSeen fields choice) := by
  intro wei
  rw [gasKeyFresh_mark_iff, seen wei]
  cases choice <;> simp [gasChoiceSeen, List.mem_cons, and_comm, eq_comm]

theorem gasSeen_fresh (mask : w64) (fields : List Bool) (choice : Option Bool) (seen : GasSeen mask fields)
    (fresh : GasKeyFresh mask choice) : GasChoiceFresh fields choice := by
  cases choice with
  | none => trivial
  | some wei => exact (seen wei).mp fresh

theorem headerSeen_fresh (mask : w64) (fields : List HeaderField) (choice : Option HeaderField)
    (seen : HeaderSeen mask fields) (fresh : HeaderKeyFresh mask choice) : HeaderChoiceFresh fields choice := by
  cases choice with
  | none => trivial
  | some field => exact (seen field).mp fresh

/-- Existing successful child-value witnesses satisfy the complete value relation. -/
theorem gasPriceEntry_value_valid (d : Nat) (entry : GasPriceEntry d) (initial : GasPriceValue) :
    GasValueValid d entry.key.choice initial entry.valueBytes (entry.apply initial) := by
  cases entry with
  | amount wei key choice value =>
    simp only [GasPriceEntry.key, GasPriceEntry.valueBytes, GasPriceEntry.apply, choice]
    exact .amount value.valid
  | ignored key choice value valid =>
    simp only [GasPriceEntry.key, GasPriceEntry.valueBytes, GasPriceEntry.apply, choice]
    exact .ignored valid

/-- Lift the complete production-proved GasPrice entry family into the new
    semantic pair relation, preserving byte order, field freshness and values. -/
theorem gasEntries_supported_spec (d : Nat) (entries : List (GasPriceEntry d))
    (mask : w64) (fields : List Bool) (initial : GasPriceValue)
    (bound : (entries.flatMap GasPriceEntry.bytes).length < 2^64)
    (seen : GasSeen mask fields) (fresh : GasEntriesFresh mask entries) :
    GasPairsValid d entries.length fields initial (entries.flatMap GasPriceEntry.bytes)
      (entries.foldl GasPriceEntry.apply initial) := by
  induction entries generalizing mask fields initial with
  | nil => exact .nil fields initial
  | cons entry entries ih =>
    have keyBound : entry.key.bytes.length < 2^64 := by
      simp only [List.flatMap_cons, List.length_append, GasPriceEntry.bytes] at bound
      omega
    have restBound : (entries.flatMap GasPriceEntry.bytes).length < 2^64 := by
      simp only [List.flatMap_cons, List.length_append] at bound
      omega
    have keyValid := gasKeySupported_valid entry.key.bytes entry.key.choice keyBound entry.key.valid
    have valueValid := gasPriceEntry_value_valid d entry initial
    have isFresh := gasSeen_fresh mask fields entry.key.choice seen fresh.1
    have rest := ih (gasKeySeen mask entry.key.choice) (gasChoiceSeen fields entry.key.choice)
      (entry.apply initial) restBound (gasSeen_mark mask fields entry.key.choice seen) fresh.2
    simpa only [List.length_cons, List.flatMap_cons, List.foldl_cons, GasPriceEntry.bytes, List.append_assoc] using
      GasPairsValid.step keyValid isFresh valueValid rest

/-- The already-proved nullable GasPrice encodings preserve nil/non-nil meaning
    when viewed through the complete pointer specification. -/
theorem headerGasWire_valid (d : Nat) (item : HeaderGasWire d) (bound : item.bytes.length < 2^64) :
    GasPointerValid d item.bytes item.value := by
  cases item with
  | nullish head hn => exact .nullish hn
  | map head payload entries enc major depth fresh =>
    have hcount := gasPriceEntries_count_bound (d + 1) entries
    have restBound : (entries.flatMap GasPriceEntry.bytes).length < 2^64 := by
      simp only [HeaderGasWire.bytes, List.length_append, List.length_cons] at bound
      omega
    have count : uint.Z (W64 entries.length) = entries.length := by word
    exact .map enc major count depth (gasEntries_supported_spec (d + 1) entries (W64 0) []
      ⟨none, none⟩ restBound gasSeen_zero fresh)

/-- Existing Header value witnesses satisfy the complete field relation,
    including the old nullish string family and every already-proved child map. -/
theorem headerEntry_value_valid (d : Nat) (entry : HeaderEntry d) (initial : HeaderValue)
    (bound : entry.valueBytes.length < 2^64) :
    HeaderItemValid d entry.key.choice initial entry.valueBytes (entry.apply initial) := by
  cases entry with
  | felt field key choice item =>
    simp only [HeaderEntry.key, HeaderEntry.valueBytes, HeaderEntry.apply, choice]
    exact .felt field item.valid
  | scalar field key choice item =>
    simp only [HeaderEntry.key, HeaderEntry.valueBytes, HeaderEntry.apply, choice]
    exact .scalar field (item.valid _)
  | signatures key choice item =>
    simp only [HeaderEntry.key, HeaderEntry.valueBytes, HeaderEntry.apply, choice]
    exact .signatures item.valid
  | versionNull key choice head hn =>
    simp only [HeaderEntry.key, HeaderEntry.valueBytes, HeaderEntry.apply, choice]
    exact .string (.nullish hn)
  | gas second key choice item =>
    simp only [HeaderEntry.key, HeaderEntry.valueBytes, HeaderEntry.apply, choice]
    exact .gas second (headerGasWire_valid d item bound)
  | ignored key choice wire valid =>
    simp only [HeaderEntry.key, HeaderEntry.valueBytes, HeaderEntry.apply, choice]
    exact .ignored valid

theorem headerEntries_supported_spec (d : Nat) (entries : List (HeaderEntry d))
    (mask : w64) (fields : List HeaderField) (initial : HeaderValue)
    (bound : (entries.flatMap HeaderEntry.bytes).length < 2^64)
    (seen : HeaderSeen mask fields) (fresh : HeaderEntriesFresh mask entries) :
    HeaderPairsValid d entries.length fields initial (entries.flatMap HeaderEntry.bytes)
      (entries.foldl HeaderEntry.apply initial) := by
  induction entries generalizing mask fields initial with
  | nil => exact .nil fields initial
  | cons entry entries ih =>
    have keyBound : entry.key.bytes.length < 2^64 := by
      simp only [List.flatMap_cons, List.length_append, HeaderEntry.bytes] at bound
      omega
    have valueBound : entry.valueBytes.length < 2^64 := by
      simp only [List.flatMap_cons, List.length_append, HeaderEntry.bytes] at bound
      omega
    have restBound : (entries.flatMap HeaderEntry.bytes).length < 2^64 := by
      simp only [List.flatMap_cons, List.length_append] at bound
      omega
    have keyValid := headerKeySupported_valid entry.key.bytes entry.key.choice keyBound entry.key.valid
    have valueValid := headerEntry_value_valid d entry initial valueBound
    have isFresh := headerSeen_fresh mask fields entry.key.choice seen fresh.1
    have marked : HeaderSeen (headerKeySeen mask entry.key.choice) (headerChoiceSeen fields entry.key.choice) :=
      headerSeen_mark mask fields entry.key.choice seen
    have rest := ih (headerKeySeen mask entry.key.choice) (headerChoiceSeen fields entry.key.choice)
      (entry.apply initial) restBound marked fresh.2
    simpa only [List.length_cons, List.flatMap_cons, List.foldl_cons, HeaderEntry.bytes, List.append_assoc] using
      HeaderPairsValid.step keyValid isFresh valueValid rest

/-- Every successful Header map already covered by a production theorem is a
    successful record in the full specification, with exactly the same value. -/
theorem headerMapWire_valid (d : Nat) (item : HeaderMapWire d) (bound : item.bytes.length < 2^64) :
    HeaderRecordValid d headerZeroValue item.bytes item.value := by
  have hcount := headerEntries_count_bound (d + 1) item.entries
  have restBound : (item.entries.flatMap HeaderEntry.bytes).length < 2^64 := by
    rw [headerMapWire_length] at bound
    omega
  have count : uint.Z (W64 item.entries.length) = item.entries.length := by word
  exact .map item.enc item.major count item.bound
    (headerEntries_supported_spec (d + 1) item.entries (W64 0) [] headerZeroValue restBound headerSeen_zero item.fresh)

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.gasKeyMasks_disjoint
#print axioms Perennial.github_com.NethermindEth.juno.core.gasKeyFresh_mark_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.gasSeen_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.gasSeen_mark
#print axioms Perennial.github_com.NethermindEth.juno.core.gasSeen_fresh
#print axioms Perennial.github_com.NethermindEth.juno.core.headerSeen_fresh
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPriceEntry_value_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.gasEntries_supported_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasWire_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.headerEntry_value_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.headerEntries_supported_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.headerMapWire_valid
