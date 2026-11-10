import ProofCheck.HeaderKeys
import ProofCheck.HeaderGasFields

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

/-- Distinct schema identities use disjoint nonzero bits in the actual mask. -/
theorem headerKeyMasks_disjoint (a b : HeaderField) :
    ((W64 1 <<< headerKeyIndex a) &&& (W64 1 <<< headerKeyIndex b) = W64 0) ↔ a ≠ b := by
  cases a <;> cases b <;> decide

/-- Marking a key removes exactly that field from the fresh set. Unknown keys
    leave the set unchanged; no unrelated field becomes seen. -/
theorem headerKeyFresh_mark_iff (seen : w64) (choice : Option HeaderField) (field : HeaderField) :
    HeaderKeyFresh (headerKeySeen seen choice) (some field) ↔
      HeaderKeyFresh seen (some field) ∧ choice ≠ some field := by
  cases choice with
  | none => simp [headerKeySeen, HeaderKeyFresh]
  | some found =>
    have hor (a b : w64) : a ||| b = W64 0 ↔ a = W64 0 ∧ b = W64 0 := BitVec.or_eq_zero_iff
    simp only [headerKeySeen, HeaderKeyFresh, BitVec.and_or_distrib_right, hor,
      headerKeyMasks_disjoint, ne_eq, Option.some.injEq]

/-- Seen bits correspond to schema identities, independently of key spelling,
    cached lookup order, or which entries were unknown keys. -/
def HeaderSeen (seen : w64) (fields : List HeaderField) : Prop :=
  ∀ field, HeaderKeyFresh seen (some field) ↔ field ∉ fields

theorem headerSeen_zero : HeaderSeen (W64 0) [] := by
  intro field
  simp [HeaderKeyFresh, BitVec.zero_and]

theorem headerSeen_mark (seen : w64) (fields : List HeaderField) (choice : Option HeaderField)
    (hs : HeaderSeen seen fields) :
    HeaderSeen (headerKeySeen seen choice) (match choice with | none => fields | some field => field :: fields) := by
  intro field
  rw [headerKeyFresh_mark_iff, hs field]
  cases choice <;> simp [List.mem_cons, and_comm, eq_comm]

def headerGasKey (second : Bool) : HeaderField := if second then .l2Gas else .dataGas

/-- A never-seen GasPrice field still owns an unpublished zero slot. This
    connects duplicate bookkeeping to the initial value used by the child map.
    Already-seen slots may contain any published or private partial value. -/
def HeaderGasUnseen (seen : w64) (snapshot : HeaderGasSnapshot) : Prop :=
  ∀ second, HeaderKeyFresh seen (some (headerGasKey second)) →
    snapshot.value second = ⟨none, none⟩ ∧ snapshot.published second = false

theorem headerGasUnseen_zero (seen : w64) : HeaderGasUnseen seen headerGasZeroSnapshot := by
  intro second _
  cases second <;> exact ⟨rfl, rfl⟩

/-- Recording any key, with no change to allocation contents, preserves the
    never-seen-slot invariant. This includes all non-price field calls. -/
theorem headerGasUnseen_mark (seen : w64) (snapshot : HeaderGasSnapshot) (choice : Option HeaderField)
    (hs : HeaderGasUnseen seen snapshot) : HeaderGasUnseen (headerKeySeen seen choice) snapshot := by
  intro second fresh
  exact hs second ((headerKeyFresh_mark_iff seen choice (headerGasKey second)).mp fresh).1

/-- Once its key is marked, a selected price slot may hold either a complete
    value or a partial failed value, and may independently be published or not.
    The other unseen slot remains zero and unpublished. -/
theorem headerGasUnseen_write (seen : w64) (snapshot : HeaderGasSnapshot) (second : Bool)
    (value : GasPriceValue) (publish : Bool) (hs : HeaderGasUnseen seen snapshot) :
    HeaderGasUnseen (headerKeySeen seen (some (headerGasKey second))) (snapshot.update second value publish) := by
  intro other fresh
  obtain ⟨hprev, hne⟩ := (headerKeyFresh_mark_iff seen (some (headerGasKey second)) (headerGasKey other)).mp fresh
  have hzero := hs other hprev
  cases second <;> cases other <;>
    simp_all [headerGasKey, HeaderGasSnapshot.update, HeaderGasSnapshot.value, HeaderGasSnapshot.published]

/-- Stronger private ownership for the eventual Header loop: besides preserving
    the shared pair, it relates unvisited fields to their zero initial storage. -/
def HeaderGasReady (base : Loc) (h : Header) (old : HeaderValue) (seen : w64) : IProp GF := iprop(
  (⌜base = null ∧ h.L1DataGasPrice' = null ∧ h.L2GasPrice' = null ∧
     old.l1DataGasPrice = none ∧ old.l2GasPrice = none⌝) ∨
  (∃ (prices : GoArray GasPrice 2) (snapshot : HeaderGasSnapshot),
    GasPricesOwned base prices snapshot.values ∗
    ⌜snapshot.layout (GF := GF) base h old ∧ HeaderGasUnseen seen snapshot⌝))

theorem headerGasReady_zero (seen : w64) :
    ⊢ (HeaderGasReady null (zero_val Header) headerZeroValue seen : IProp GF) := by
  unfold HeaderGasReady
  ileft
  ipureintro
  exact ⟨rfl, rfl, rfl, rfl, rfl⟩

theorem headerGasReady_snapshot (base : Loc) (prices : GoArray GasPrice 2) (h : Header) (old : HeaderValue)
    (seen : w64) (snapshot : HeaderGasSnapshot) (layout : snapshot.layout (GF := GF) base h old)
    (unseen : HeaderGasUnseen seen snapshot) :
    GasPricesOwned base prices snapshot.values ⊢ (HeaderGasReady base h old seen : IProp GF) := by
  iintro Hprices
  unfold HeaderGasReady
  iright
  iexists prices, snapshot
  iframe Hprices
  ipureintro
  exact ⟨layout, unseen⟩

/-- The stronger invariant retains the existing publication and cleanup
    contracts; it does not add or duplicate any heap ownership. -/
theorem headerGasReady_forget (base : Loc) (h : Header) (old : HeaderValue) (seen : w64) :
    HeaderGasReady base h old seen ⊢ (HeaderGasState base h old : IProp GF) := by
  unfold HeaderGasReady
  iintro Hready
  icases Hready with (Hnil | Hshared)
  · unfold HeaderGasState
    ileft
    iexact Hnil
  · icases Hshared with ⟨%prices, %snapshot, Hprices, %Hlayout⟩
    iapply headerGasState_snapshot base prices h old snapshot Hlayout.1
    iexact Hprices

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerKeyMasks_disjoint
#print axioms Perennial.github_com.NethermindEth.juno.core.headerKeyFresh_mark_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.headerSeen_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.headerSeen_mark
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasUnseen_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasUnseen_mark
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasUnseen_write
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasReady_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasReady_snapshot
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasReady_forget
