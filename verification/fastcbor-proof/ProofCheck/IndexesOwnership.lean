import ProofCheck.IndexesRecordSpec
import ProofCheck.IntsComplete

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

def IndexesChildren (h : BlockTransactionsIndexes) (value : IndexesValue) : IProp GF := iprop(
  IntsResult h.Transactions' value.transactions ∗ IntsResult h.Receipts' value.receipts)

def IndexesOwned (dst : Loc) (value : IndexesValue) : IProp GF := iprop(
  ∃ h : BlockTransactionsIndexes, dst ↦ h ∗ IndexesChildren h value)

def indexesSlice (h : BlockTransactionsIndexes) (field : Bool) : GoSlice :=
  if field then h.Receipts' else h.Transactions'

def indexesStore (h : BlockTransactionsIndexes) (field : Bool) (p : GoSlice) : BlockTransactionsIndexes :=
  if field then {h with Receipts' := p} else {h with Transactions' := p}

def indexesFieldRef (dst : Loc) (field : Bool) : Loc :=
  if field then structFieldRef BlockTransactionsIndexes go!"Receipts" dst
  else structFieldRef BlockTransactionsIndexes go!"Transactions" dst

/-- Borrow either generated field while retaining the other concrete slice. -/
theorem indexesField_acc (dst : Loc) (h : BlockTransactionsIndexes) (field : Bool) :
    dst ↦ h ⊢ (iprop(indexesFieldRef (GF := GF) dst field ↦ indexesSlice h field ∗
      (∀ p : GoSlice, indexesFieldRef (GF := GF) dst field ↦ p -∗ dst ↦ indexesStore h field p)) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup _ _ _ $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split _ _ _ $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Htransactions, Hreceipts, -⟩
  cases field <;> isimp only [indexesFieldRef, indexesSlice, indexesStore, Bool.false_eq_true, ite_false, ite_true]
  · iframe Htransactions
    iintro %p Hvalue
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef]
    iframe
  · iframe Hreceipts
    iintro %p Hvalue
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef]
    iframe

/-- Publishing a replacement retains independent ownership of the other array. -/
theorem indexesChildren_set (h : BlockTransactionsIndexes) (old : IndexesValue)
    (field : Bool) (p : GoSlice) (value : Option (List Int)) :
    IndexesChildren h old ∗ IntsResult p value ⊢
      (IndexesChildren (indexesStore h field p) (indexesSet old field value) : IProp GF) := by
  cases field <;> unfold IndexesChildren indexesStore indexesSet <;>
    isimp only [Bool.false_eq_true, ite_false, ite_true] <;>
    iintro ⟨⟨Htransactions, Hreceipts⟩, Hvalue⟩ <;> iframe

theorem intsResult_nil_iff (out : GoSlice) (value : Option (List Int)) :
    IntsResult (GF := GF) out value ⊢ ⌜out = slice.nil ↔ value = none⌝ := by
  cases value with
  | none =>
    unfold IntsResult
    iintro %hs
    ipureintro
    simp [hs]
  | some values =>
    unfold IntsResult
    iintro ⟨%words, _, %hn, %hvalues⟩
    have hs : out ≠ slice.nil := by intro h; subst out; exact hn rfl
    ipureintro
    simp [hs]

theorem indexesChildren_zero_iff (h : BlockTransactionsIndexes) (value : IndexesValue) :
    IndexesChildren (GF := GF) h value ⊢ ⌜h = zero_val BlockTransactionsIndexes ↔ value = indexesZeroValue⌝ := by
  unfold IndexesChildren
  iintro ⟨Htransactions, Hreceipts⟩
  ihave %Ht := intsResult_nil_iff h.Transactions' value.transactions $$ Htransactions
  ihave %Hr := intsResult_nil_iff h.Receipts' value.receipts $$ Hreceipts
  ipureintro
  constructor
  · intro hz
    subst h
    have ht := Ht.mp rfl
    have hr := Hr.mp rfl
    cases value
    simp_all [indexesZeroValue]
  · intro hz
    subst value
    have ht := Ht.mpr rfl
    have hr := Hr.mpr rfl
    cases h
    simp_all
    rfl

/-- Arbitrary supported array success at either actual generated field. -/
theorem wp_indexesField_valid (c dst : Loc) (h : BlockTransactionsIndexes) (old : IndexesValue)
    (field : Bool) (v : cursor.Cursor) (xs wire rem : List w8) (value : Option (List Int)) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32) (valid : IntsValid d wire value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ h ∗ IndexesChildren h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadInts)) (Val #c)) (Val #(indexesFieldRef (GF := GF) dst field)))
    {{ (out : GoSlice), RET #interface.nil;
       dst ↦ indexesStore h field out ∗ IndexesChildren (indexesStore h field out) (indexesSet old field value) ∗
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨Hh, Hchildren, Hc, Hs⟩
  icases indexesField_acc dst h field $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readInts_valid c (indexesFieldRef (GF := GF) dst field) (indexesSlice h field) v xs wire rem value dq d
    hd hdepth valid hp hbytes $$ [Hfield Hc Hs] as %out ⟨Hfield, Hvalue, Hc, Hs, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Hh := Hclose $$ %out Hfield
  ihave Hchildren := indexesChildren_set h old field out value $$ [Hchildren Hvalue]
  · iframe Hchildren Hvalue
  iapply HΦ
  iframe Hh Hchildren Hc Hs
  ipureintro
  exact Hnext

/-- Every failed array replacement preserves both old published arrays, even
    after reading a valid prefix into private storage or advancing on overflow. -/
theorem wp_indexesField_failure (c dst : Loc) (h : BlockTransactionsIndexes) (old : IndexesValue)
    (field : Bool) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d consumed offset : Nat) (entered : Bool)
    (reason : IntsFailureReason) (k : BigIntErrorKinds) (depthKind : Loc)
    (failure : IntsRejected d wire consumed offset entered reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       BigIntErrorGlobals k ∗ globalAddr cursor.ErrDepth ↦□ bigIntCategory depthKind ∗
       dst ↦ h ∗ IndexesChildren h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadInts)) (Val #c)) (Val #(indexesFieldRef (GF := GF) dst field)))
    {{ (result : GoInterface), RET #result; dst ↦ h ∗ IndexesChildren h old ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 (if entered then 1 else 0 : Nat)} ∗
       v.buf' ↦*{dq} xs ∗ IntsFailureError result (v.pos' + W64 offset) reason k depthKind }} := by
  wp_start_folded as ⟨#Hglobals, #Hdepth, Hh, Hchildren, Hc, Hs⟩
  icases indexesField_acc dst h field $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readInts_failure c (indexesFieldRef (GF := GF) dst field) (indexesSlice h field) v xs wire dq d consumed offset entered
    reason k depthKind failure hd hdepth hp hbytes $$ [Hfield Hc Hs] as %result ⟨Hfield, Hc, Hs, Herr⟩
  · iframe Hglobals Hdepth Hfield Hc Hs
  ihave Hh := Hclose $$ %(indexesSlice h field) Hfield
  have heq : indexesStore h field (indexesSlice h field) = h := by cases field <;> rfl
  isimp only [heq] at Hh
  iapply HΦ
  iframe Hh Hchildren Hc Hs Herr

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesField_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesChildren_set
#print axioms Perennial.github_com.NethermindEth.juno.core.intsResult_nil_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesChildren_zero_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_indexesField_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_indexesField_failure
