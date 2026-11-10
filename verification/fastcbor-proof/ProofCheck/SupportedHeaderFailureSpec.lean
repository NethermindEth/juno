import ProofCheck.SupportedFailureSpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Preserve the exact successful prefix and retained value before a value error. -/
theorem headerEntries_value_rejected (d remaining : Nat) (entries : List (HeaderEntry d))
    (mask : w64) (fields : List HeaderField) (initial : HeaderValue)
    (tail : List w8) (key : HeaderKeyWire) (consumed offset extraDepth : Nat) (reason : HeaderValueFailureReason)
    (keyBound : key.bytes.length < 2^64)
    (before : HeaderEntriesBefore mask entries key.choice)
    (failure : HeaderItemRejected d key.choice tail consumed offset extraDepth reason)
    (bound : (entries.flatMap HeaderEntry.bytes).length < 2^64) (seen : HeaderSeen mask fields) :
    HeaderPairsRejected d (entries.length + remaining + 1) fields initial
      (entries.flatMap HeaderEntry.bytes ++ key.bytes ++ tail) (entries.foldl HeaderEntry.apply initial)
      ((entries.flatMap HeaderEntry.bytes).length + key.bytes.length + consumed) ((entries.flatMap HeaderEntry.bytes).length + key.bytes.length + offset) extraDepth (.value reason) := by
  induction entries generalizing mask fields initial with
  | nil =>
    simpa only [List.length_nil, Nat.zero_add, List.flatMap_nil, List.nil_append, List.foldl_nil] using
      (HeaderPairsRejected.value (n := remaining)
        (headerKeySupported_valid key.bytes key.choice keyBound key.valid)
        (headerSeen_fresh mask fields key.choice seen before) failure)
  | cons entry entries ih =>
    have kb : entry.key.bytes.length < 2^64 := by
      simp only [List.flatMap_cons, List.length_append, HeaderEntry.bytes] at bound
      omega
    have vb : entry.valueBytes.length < 2^64 := by
      simp only [List.flatMap_cons, List.length_append, HeaderEntry.bytes] at bound
      omega
    have rb : (entries.flatMap HeaderEntry.bytes).length < 2^64 := by
      simp only [List.flatMap_cons, List.length_append] at bound
      omega
    have rest := ih (headerKeySeen mask entry.key.choice) (headerChoiceSeen fields entry.key.choice)
      (entry.apply initial) before.2 rb (headerSeen_mark mask fields entry.key.choice seen)
    have result := HeaderPairsRejected.next
      (headerKeySupported_valid entry.key.bytes entry.key.choice kb entry.key.valid)
      (headerSeen_fresh mask fields entry.key.choice seen before.1)
      (headerEntry_value_valid d entry initial vb) rest
    simpa only [List.length_cons, List.flatMap_cons, List.foldl_cons, HeaderEntry.bytes,
      List.length_append, List.append_assoc, Nat.add_assoc, Nat.add_left_comm, Nat.add_comm] using result

/-- Preserve the exact successful prefix and retained value before a duplicate error. -/
theorem headerEntries_duplicate_rejected (d remaining : Nat) (entries : List (HeaderEntry d))
    (mask : w64) (fields : List HeaderField) (initial : HeaderValue)
    (tail : List w8) (key : HeaderKeyWire) (field : HeaderField)
    (keyBound : key.bytes.length < 2^64)
    (before : HeaderEntriesBeforeDuplicate mask entries field) (choice : key.choice = some field)
    (bound : (entries.flatMap HeaderEntry.bytes).length < 2^64) (seen : HeaderSeen mask fields) :
    HeaderPairsRejected d (entries.length + remaining + 1) fields initial
      (entries.flatMap HeaderEntry.bytes ++ key.bytes ++ tail) (entries.foldl HeaderEntry.apply initial)
      ((entries.flatMap HeaderEntry.bytes).length + key.bytes.length) ((entries.flatMap HeaderEntry.bytes).length + key.bytes.length) 0 (.duplicate field) := by
  induction entries generalizing mask fields initial with
  | nil =>
    have member : field ∈ fields := by
      classical
      have neq : ¬ HeaderKeyFresh mask (some field) := before
      exact Classical.byContradiction (fun h => neq ((seen field).mpr h))
    have valid : HeaderKeyValid key.bytes (some field) := by
      simpa only [choice] using headerKeySupported_valid key.bytes key.choice keyBound key.valid
    simpa only [List.length_nil, Nat.zero_add, List.flatMap_nil, List.nil_append, List.foldl_nil] using
      (HeaderPairsRejected.duplicate (n := remaining) valid member)
  | cons entry entries ih =>
    have kb : entry.key.bytes.length < 2^64 := by
      simp only [List.flatMap_cons, List.length_append, HeaderEntry.bytes] at bound
      omega
    have vb : entry.valueBytes.length < 2^64 := by
      simp only [List.flatMap_cons, List.length_append, HeaderEntry.bytes] at bound
      omega
    have rb : (entries.flatMap HeaderEntry.bytes).length < 2^64 := by
      simp only [List.flatMap_cons, List.length_append] at bound
      omega
    have rest := ih (headerKeySeen mask entry.key.choice) (headerChoiceSeen fields entry.key.choice)
      (entry.apply initial) before.2 rb (headerSeen_mark mask fields entry.key.choice seen)
    have result := HeaderPairsRejected.next
      (headerKeySupported_valid entry.key.bytes entry.key.choice kb entry.key.valid)
      (headerSeen_fresh mask fields entry.key.choice seen before.1)
      (headerEntry_value_valid d entry initial vb) rest
    simpa only [List.length_cons, List.flatMap_cons, List.foldl_cons, HeaderEntry.bytes,
      List.length_append, List.append_assoc, Nat.add_assoc, Nat.add_left_comm, Nat.add_comm] using result

/-- Preserve the exact successful prefix and retained value before a key error. -/
theorem headerEntries_key_rejected (d remaining : Nat) (entries : List (HeaderEntry d))
    (mask : w64) (fields : List HeaderField) (initial : HeaderValue)
    (tail : List w8) (reason : KeyFailureReason)
    (fresh : HeaderEntriesFresh mask entries) (failure : KeyRejected tail reason)
    (bound : (entries.flatMap HeaderEntry.bytes).length < 2^64) (seen : HeaderSeen mask fields) :
    HeaderPairsRejected d (entries.length + remaining + 1) fields initial
      (entries.flatMap HeaderEntry.bytes ++ tail) (entries.foldl HeaderEntry.apply initial)
      ((entries.flatMap HeaderEntry.bytes).length) ((entries.flatMap HeaderEntry.bytes).length) 0 (.key reason) := by
  induction entries generalizing mask fields initial with
  | nil =>
    simpa only [List.length_nil, Nat.zero_add, List.flatMap_nil, List.nil_append, List.foldl_nil] using
      (HeaderPairsRejected.key (d := d) (n := remaining) (seen := fields) (initial := initial) failure)
  | cons entry entries ih =>
    have kb : entry.key.bytes.length < 2^64 := by
      simp only [List.flatMap_cons, List.length_append, HeaderEntry.bytes] at bound
      omega
    have vb : entry.valueBytes.length < 2^64 := by
      simp only [List.flatMap_cons, List.length_append, HeaderEntry.bytes] at bound
      omega
    have rb : (entries.flatMap HeaderEntry.bytes).length < 2^64 := by
      simp only [List.flatMap_cons, List.length_append] at bound
      omega
    have rest := ih (headerKeySeen mask entry.key.choice) (headerChoiceSeen fields entry.key.choice)
      (entry.apply initial) fresh.2 rb (headerSeen_mark mask fields entry.key.choice seen)
    have result := HeaderPairsRejected.next
      (headerKeySupported_valid entry.key.bytes entry.key.choice kb entry.key.valid)
      (headerSeen_fresh mask fields entry.key.choice seen fresh.1)
      (headerEntry_value_valid d entry initial vb) rest
    simpa only [List.length_cons, List.flatMap_cons, List.foldl_cons, HeaderEntry.bytes,
      List.length_append, List.append_assoc, Nat.add_assoc, Nat.add_left_comm, Nat.add_comm] using result

def headerLookupRecordReason (reason : HeaderLookupFailureReason) : HeaderRecordFailureReason :=
  match reason with
  | .duplicate field => .duplicate field
  | .key reason => .key reason

/-- Legacy key/duplicate failures retain exactly the same prefix value and
    progress in the complete Header record relation. -/
theorem headerLookupFailure_rejected (d offset : Nat) (wire : List w8) (value : HeaderValue)
    (reason : HeaderLookupFailureReason) (bound : wire.length < 2^64)
    (failure : HeaderLookupFailure d wire value offset reason) :
    HeaderRecordRejected d headerZeroValue wire value offset offset 1 (headerLookupRecordReason reason) := by
  cases failure with
  | @duplicate count head payload tail field entries key enc major depth more fit before choice =>
    have eb : (entries.flatMap HeaderEntry.bytes).length < 2^64 := by
      simp only [List.length_append, List.length_cons] at bound; omega
    have kb : key.bytes.length < 2^64 := by
      simp only [List.length_append, List.length_cons] at bound; omega
    have len : uint.Z (W64 count) = count := by
      simp only [List.length_append, List.length_cons] at bound fit
      word
    have pairs := headerEntries_duplicate_rejected (d + 1) (count - entries.length - 1) entries
      (W64 0) [] headerZeroValue tail key field kb before choice eb headerSeen_zero
    have countEq : entries.length + (count - entries.length - 1) + 1 = count := by omega
    rw [countEq] at pairs
    simpa only [headerLookupRecordReason, List.append_assoc, Nat.add_assoc] using
      HeaderRecordRejected.map enc major len depth fit pairs
  | @key count head payload tail reason entries enc major depth more fit fresh failure =>
    have eb : (entries.flatMap HeaderEntry.bytes).length < 2^64 := by
      simp only [List.length_append, List.length_cons] at bound; omega
    have tb : tail.length < 2^64 := by
      simp only [List.length_append, List.length_cons] at bound; omega
    have len : uint.Z (W64 count) = count := by
      simp only [List.length_append, List.length_cons] at bound fit
      word
    have pairs := headerEntries_key_rejected (d + 1) (count - entries.length - 1) entries
      (W64 0) [] headerZeroValue tail reason fresh (keyFailure_rejected tail reason tb failure) eb headerSeen_zero
    have countEq : entries.length + (count - entries.length - 1) + 1 = count := by omega
    rw [countEq] at pairs
    simpa only [headerLookupRecordReason, List.append_assoc, Nat.add_assoc] using
      HeaderRecordRejected.map enc major len depth fit pairs

/-- Whole-record field failures embed with every prior update, exact consumed
    count and diagnostic, and retained depth of nested signature/GasPrice errors. -/
theorem headerValueFailureWire_rejected (d : Nat) (item : HeaderValueFailureWire d)
    (bound : item.bytes.length < 2^64) :
    HeaderRecordRejected d headerZeroValue item.bytes
      (item.entries.foldl HeaderEntry.apply headerZeroValue)
      (item.valueStart + item.consumed) item.errorOffset (1 + item.extraDepth) (.value item.reason) := by
  have eb : (item.entries.flatMap HeaderEntry.bytes).length < 2^64 := by
    simp only [HeaderValueFailureWire.bytes, List.length_append, List.length_cons] at bound; omega
  have kb : item.key.bytes.length < 2^64 := by
    simp only [HeaderValueFailureWire.bytes, List.length_append, List.length_cons] at bound; omega
  have tb : item.tail.length < 2^64 := by
    simp only [HeaderValueFailureWire.bytes, List.length_append, List.length_cons] at bound; omega
  have len : uint.Z (W64 item.count) = item.count := by
    have fit := item.fit
    simp only [HeaderValueFailureWire.bytes, List.length_append, List.length_cons] at bound fit
    word
  have pairs := headerEntries_value_rejected (d + 1) (item.count - item.entries.length - 1) item.entries
    (W64 0) [] headerZeroValue item.tail item.key item.consumed item.offset item.extraDepth item.reason kb item.fresh
    (headerValueFailure_rejected _ _ _ _ _ _ _ tb item.failure) eb headerSeen_zero
  have countEq : item.entries.length + (item.count - item.entries.length - 1) + 1 = item.count := by
    have := item.more; omega
  rw [countEq] at pairs
  simpa only [HeaderValueFailureWire.bytes, HeaderValueFailureWire.valueStart, HeaderValueFailureWire.errorOffset,
    List.append_assoc, Nat.add_assoc] using
    HeaderRecordRejected.map item.enc item.major len item.bound item.fit pairs

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerEntries_value_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.headerEntries_duplicate_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.headerEntries_key_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.headerLookupFailure_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.headerValueFailureWire_rejected
