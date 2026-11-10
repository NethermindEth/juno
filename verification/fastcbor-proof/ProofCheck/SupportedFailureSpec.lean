import ProofCheck.SupportedSpec
import ProofCheck.HeaderLookupFailure

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Preserve the exact successful prefix and retained value before a value error. -/
theorem gasEntries_value_rejected (d remaining : Nat) (entries : List (GasPriceEntry d))
    (mask : w64) (fields : List Bool) (initial : GasPriceValue)
    (tail : List w8) (key : GasKeyWire) (offset : Nat) (reason : FeltFailureReason)
    (keyBound : key.bytes.length < 2^64)
    (before : GasEntriesBefore mask entries key.choice)
    (failure : GasPriceValueFailure d key.choice tail offset reason)
    (bound : (entries.flatMap GasPriceEntry.bytes).length < 2^64) (seen : GasSeen mask fields) :
    GasPairsRejected d (entries.length + remaining + 1) fields initial
      (entries.flatMap GasPriceEntry.bytes ++ key.bytes ++ tail) (entries.foldl GasPriceEntry.apply initial)
      ((entries.flatMap GasPriceEntry.bytes).length + key.bytes.length) ((entries.flatMap GasPriceEntry.bytes).length + key.bytes.length + offset) (.inner reason) := by
  induction entries generalizing mask fields initial with
  | nil =>
    simpa only [List.length_nil, Nat.zero_add, List.flatMap_nil, List.nil_append, List.foldl_nil] using
      (GasPairsRejected.value (n := remaining)
        (gasKeySupported_valid key.bytes key.choice keyBound key.valid)
        (gasSeen_fresh mask fields key.choice seen before) failure)
  | cons entry entries ih =>
    have kb : entry.key.bytes.length < 2^64 := by
      simp only [List.flatMap_cons, List.length_append, GasPriceEntry.bytes] at bound
      omega
    have rb : (entries.flatMap GasPriceEntry.bytes).length < 2^64 := by
      simp only [List.flatMap_cons, List.length_append] at bound
      omega
    have rest := ih (gasKeySeen mask entry.key.choice) (gasChoiceSeen fields entry.key.choice)
      (entry.apply initial) before.2 rb (gasSeen_mark mask fields entry.key.choice seen)
    have result := GasPairsRejected.next
      (gasKeySupported_valid entry.key.bytes entry.key.choice kb entry.key.valid)
      (gasSeen_fresh mask fields entry.key.choice seen before.1)
      (gasPriceEntry_value_valid d entry initial) rest
    simpa only [List.length_cons, List.flatMap_cons, List.foldl_cons, GasPriceEntry.bytes,
      List.length_append, List.append_assoc, Nat.add_assoc, Nat.add_left_comm, Nat.add_comm] using result

/-- Preserve the exact successful prefix and retained value before a duplicate error. -/
theorem gasEntries_duplicate_rejected (d remaining : Nat) (entries : List (GasPriceEntry d))
    (mask : w64) (fields : List Bool) (initial : GasPriceValue)
    (tail : List w8) (key : GasKeyWire) (wei : Bool)
    (keyBound : key.bytes.length < 2^64)
    (before : GasEntriesBeforeDuplicate mask entries wei) (choice : key.choice = some wei)
    (bound : (entries.flatMap GasPriceEntry.bytes).length < 2^64) (seen : GasSeen mask fields) :
    GasPairsRejected d (entries.length + remaining + 1) fields initial
      (entries.flatMap GasPriceEntry.bytes ++ key.bytes ++ tail) (entries.foldl GasPriceEntry.apply initial)
      ((entries.flatMap GasPriceEntry.bytes).length + key.bytes.length) ((entries.flatMap GasPriceEntry.bytes).length + key.bytes.length) (.duplicate wei) := by
  induction entries generalizing mask fields initial with
  | nil =>
    have member : wei ∈ fields := by
      classical
      have neq : ¬ GasKeyFresh mask (some wei) := before
      exact Classical.byContradiction (fun h => neq ((seen wei).mpr h))
    have valid : GasKeyValid key.bytes (some wei) := by
      simpa only [choice] using gasKeySupported_valid key.bytes key.choice keyBound key.valid
    simpa only [List.length_nil, Nat.zero_add, List.flatMap_nil, List.nil_append, List.foldl_nil] using
      (GasPairsRejected.duplicate (n := remaining) valid member)
  | cons entry entries ih =>
    have kb : entry.key.bytes.length < 2^64 := by
      simp only [List.flatMap_cons, List.length_append, GasPriceEntry.bytes] at bound
      omega
    have rb : (entries.flatMap GasPriceEntry.bytes).length < 2^64 := by
      simp only [List.flatMap_cons, List.length_append] at bound
      omega
    have rest := ih (gasKeySeen mask entry.key.choice) (gasChoiceSeen fields entry.key.choice)
      (entry.apply initial) before.2 rb (gasSeen_mark mask fields entry.key.choice seen)
    have result := GasPairsRejected.next
      (gasKeySupported_valid entry.key.bytes entry.key.choice kb entry.key.valid)
      (gasSeen_fresh mask fields entry.key.choice seen before.1)
      (gasPriceEntry_value_valid d entry initial) rest
    simpa only [List.length_cons, List.flatMap_cons, List.foldl_cons, GasPriceEntry.bytes,
      List.length_append, List.append_assoc, Nat.add_assoc, Nat.add_left_comm, Nat.add_comm] using result

/-- Preserve the exact successful prefix and retained value before a key error. -/
theorem gasEntries_key_rejected (d remaining : Nat) (entries : List (GasPriceEntry d))
    (mask : w64) (fields : List Bool) (initial : GasPriceValue)
    (tail : List w8) (reason : KeyFailureReason)
    (fresh : GasEntriesFresh mask entries) (failure : KeyRejected tail reason)
    (bound : (entries.flatMap GasPriceEntry.bytes).length < 2^64) (seen : GasSeen mask fields) :
    GasPairsRejected d (entries.length + remaining + 1) fields initial
      (entries.flatMap GasPriceEntry.bytes ++ tail) (entries.foldl GasPriceEntry.apply initial)
      ((entries.flatMap GasPriceEntry.bytes).length) ((entries.flatMap GasPriceEntry.bytes).length) (.key reason) := by
  induction entries generalizing mask fields initial with
  | nil =>
    simpa only [List.length_nil, Nat.zero_add, List.flatMap_nil, List.nil_append, List.foldl_nil] using
      (GasPairsRejected.key (d := d) (n := remaining) (seen := fields) (initial := initial) failure)
  | cons entry entries ih =>
    have kb : entry.key.bytes.length < 2^64 := by
      simp only [List.flatMap_cons, List.length_append, GasPriceEntry.bytes] at bound
      omega
    have rb : (entries.flatMap GasPriceEntry.bytes).length < 2^64 := by
      simp only [List.flatMap_cons, List.length_append] at bound
      omega
    have rest := ih (gasKeySeen mask entry.key.choice) (gasChoiceSeen fields entry.key.choice)
      (entry.apply initial) fresh.2 rb (gasSeen_mark mask fields entry.key.choice seen)
    have result := GasPairsRejected.next
      (gasKeySupported_valid entry.key.bytes entry.key.choice kb entry.key.valid)
      (gasSeen_fresh mask fields entry.key.choice seen fresh.1)
      (gasPriceEntry_value_valid d entry initial) rest
    simpa only [List.length_cons, List.flatMap_cons, List.foldl_cons, GasPriceEntry.bytes,
      List.length_append, List.append_assoc, Nat.add_assoc, Nat.add_left_comm, Nat.add_comm] using result

/-- Every currently production-proved GasPrice failure has the same retained
    value, cursor progress and diagnostic in the complete record specification. -/
theorem gasPriceFailure_rejected (d consumed offset : Nat) (entered : Bool) (wire : List w8)
    (initial final : GasPriceValue) (reason : GasPriceFailureReason)
    (bound : wire.length < 2^64)
    (failure : GasPriceFailure d wire initial final consumed offset entered reason) :
    GasRecordRejected d initial wire final consumed offset entered reason := by
  cases failure with
  | opening initial failure => exact .opening failure
  | @value count offset head payload tail reason initial entries key enc major depth more fit fresh failure =>
    have eb : (entries.flatMap GasPriceEntry.bytes).length < 2^64 := by
      simp only [List.length_append, List.length_cons] at bound; omega
    have kb : key.bytes.length < 2^64 := by
      simp only [List.length_append, List.length_cons] at bound; omega
    have len : uint.Z (W64 count) = count := by
      simp only [List.length_append, List.length_cons] at bound fit
      word
    have pairs := gasEntries_value_rejected (d + 1) (count - entries.length - 1) entries
      (W64 0) [] initial tail key offset reason kb fresh failure eb gasSeen_zero
    have countEq : entries.length + (count - entries.length - 1) + 1 = count := by omega
    rw [countEq] at pairs
    simpa only [List.append_assoc, Nat.add_assoc] using
      GasRecordRejected.map enc major len depth fit pairs
  | @duplicate count head payload tail wei initial entries key enc major depth more fit before choice =>
    have eb : (entries.flatMap GasPriceEntry.bytes).length < 2^64 := by
      simp only [List.length_append, List.length_cons] at bound; omega
    have kb : key.bytes.length < 2^64 := by
      simp only [List.length_append, List.length_cons] at bound; omega
    have len : uint.Z (W64 count) = count := by
      simp only [List.length_append, List.length_cons] at bound fit
      word
    have pairs := gasEntries_duplicate_rejected (d + 1) (count - entries.length - 1) entries
      (W64 0) [] initial tail key wei kb before choice eb gasSeen_zero
    have countEq : entries.length + (count - entries.length - 1) + 1 = count := by omega
    rw [countEq] at pairs
    simpa only [List.append_assoc, Nat.add_assoc] using
      GasRecordRejected.map enc major len depth fit pairs
  | @key count head payload tail reason initial entries enc major depth more fit fresh failure =>
    have eb : (entries.flatMap GasPriceEntry.bytes).length < 2^64 := by
      simp only [List.length_append, List.length_cons] at bound; omega
    have tb : tail.length < 2^64 := by
      simp only [List.length_append, List.length_cons] at bound; omega
    have len : uint.Z (W64 count) = count := by
      simp only [List.length_append, List.length_cons] at bound fit
      word
    have pairs := gasEntries_key_rejected (d + 1) (count - entries.length - 1) entries
      (W64 0) [] initial tail reason fresh (keyFailure_rejected tail reason tb failure) eb gasSeen_zero
    have countEq : entries.length + (count - entries.length - 1) + 1 = count := by omega
    rw [countEq] at pairs
    simpa only [List.append_assoc, Nat.add_assoc] using
      GasRecordRejected.map enc major len depth fit pairs

/-- The legacy field-error family embeds without changing rollback, retained
    child depth, private GasPrice partial contents or diagnostic offsets. -/
theorem headerValueFailure_rejected (d consumed offset extraDepth : Nat) (choice : Option HeaderField)
    (wire : List w8) (reason : HeaderValueFailureReason) (bound : wire.length < 2^64)
    (failure : HeaderValueFailure d choice wire consumed offset extraDepth reason) :
    HeaderItemRejected d choice wire consumed offset extraDepth reason := by
  cases failure with
  | felt field failure => exact .felt field failure
  | scalar field failure => exact .scalar field failure
  | signatures failure => exact .signatures failure
  | string failure => exact .string (stringFailure_rejected wire _ bound failure)
  | gas second final failure => exact .gas second final (gasPriceFailure_rejected _ _ _ _ _ _ _ _ bound failure)
  | ignored failure => exact .ignored failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.gasEntries_value_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.gasEntries_duplicate_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.gasEntries_key_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPriceFailure_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.headerValueFailure_rejected
