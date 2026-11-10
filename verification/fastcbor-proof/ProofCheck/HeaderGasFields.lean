import ProofCheck.HeaderOwnership

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

/-- False selects L1DataGasPrice/slot zero; true selects L2GasPrice/slot one. -/
def headerGasName (second : Bool) : GoString := if second then go!"L2GasPrice" else go!"L1DataGasPrice"
def headerGasSlot (second : Bool) : w64 := if second then W64 1 else W64 0
def headerGasPointer (second : Bool) (h : Header) : Loc := if second then h.L2GasPrice' else h.L1DataGasPrice'
def headerSetGasPointer (second : Bool) (h : Header) (p : Loc) : Header :=
  if second then {h with L2GasPrice' := p} else {h with L1DataGasPrice' := p}
def headerSetGasValue (second : Bool) (old : HeaderValue) (value : Option GasPriceValue) : HeaderValue :=
  if second then {old with l2GasPrice := value} else {old with l1DataGasPrice := value}

/-- The private contents and publication flags of a shared pair. Unpublished
    storage can contain partial values after a failed read, without exposing them. -/
structure HeaderGasSnapshot where
  first : GasPriceValue
  second : GasPriceValue
  publishFirst : Bool
  publishSecond : Bool

def HeaderGasSnapshot.values (s : HeaderGasSnapshot) := [s.first, s.second]
def HeaderGasSnapshot.value (s : HeaderGasSnapshot) (second : Bool) := if second then s.second else s.first
def HeaderGasSnapshot.published (s : HeaderGasSnapshot) (second : Bool) := if second then s.publishSecond else s.publishFirst
def HeaderGasSnapshot.update (s : HeaderGasSnapshot) (second : Bool) (value : GasPriceValue) (publish : Bool) : HeaderGasSnapshot :=
  if second then {s with second := value, publishSecond := publish}
  else {s with first := value, publishFirst := publish}
def HeaderGasSnapshot.layout (s : HeaderGasSnapshot) (base : Loc) (h : Header) (old : HeaderValue) : Prop :=
  h.L1DataGasPrice' = (if s.publishFirst then arrayIndexRef GasPrice 0 base else null) ∧
  h.L2GasPrice' = (if s.publishSecond then arrayIndexRef GasPrice 1 base else null) ∧
  old.l1DataGasPrice = (if s.publishFirst then some s.first else none) ∧
  old.l2GasPrice = (if s.publishSecond then some s.second else none)

def headerGasZeroSnapshot : HeaderGasSnapshot := ⟨⟨none, none⟩, ⟨none, none⟩, false, false⟩

theorem headerGasSlot_bound (second : Bool) : 0 ≤ sint.Z (headerGasSlot second) ∧ sint.Z (headerGasSlot second) < 2 := by
  cases second <;> decide

theorem headerGasSnapshot_get (s : HeaderGasSnapshot) (second : Bool) :
    s.values[(sint.Z (headerGasSlot second)).toNat]? = some (s.value second) := by cases second <;> rfl

theorem headerGasSnapshot_set (s : HeaderGasSnapshot) (second : Bool) (value : GasPriceValue) (publish : Bool) :
    s.values.set (sint.Z (headerGasSlot second)).toNat value = (s.update second value publish).values := by
  cases second <;> rfl

theorem headerSetGasPointer_original (second : Bool) (h : Header) :
    headerSetGasPointer second h (headerGasPointer second h) = h := by cases second <;> rfl

theorem headerGas_acc (dst : Loc) (h : Header) (second : Bool) :
    dst ↦ h ⊢ (iprop(
      structFieldRef Header (headerGasName second) dst ↦ headerGasPointer second h ∗
      (∀ p : Loc, structFieldRef Header (headerGasName second) dst ↦ p -∗ dst ↦ headerSetGasPointer second h p)) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup _ _ _ $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split _ _ _ $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, Hparent, Hnumber, Hroot, Hsequencer, Htransactions, Hevents,
    Htime, Hversion, Heth, Hsignatures, Hstrk, Hmode, HdataGas, Hl2Gas, -⟩
  cases second <;> isimp only [headerGasName, headerGasPointer, headerSetGasPointer, Bool.false_eq_true, ite_false, ite_true]
  all_goals
    iframe
    iintro %p Hvalue
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef]
    iframe

/-- Both price fields are outside HeaderBody's ownership. -/
theorem headerBody_set_gas (h : Header) (old : HeaderValue) (second : Bool) (p : Loc) (value : Option GasPriceValue) :
    HeaderBody (GF := GF) (headerSetGasPointer second h p) (headerSetGasValue second old value) = HeaderBody h old := by
  cases second <;> rfl

theorem headerBody_set_gas_value (h : Header) (old : HeaderValue) (second : Bool) (value : Option GasPriceValue) :
    HeaderBody (GF := GF) h (headerSetGasValue second old value) = HeaderBody h old := by cases second <;> rfl

/-- Package the exact allocated state into the loop invariant, without splitting
    or duplicating either struct's ownership. -/
theorem headerGasState_snapshot (base : Loc) (prices : GoArray GasPrice 2) (h : Header) (old : HeaderValue)
    (s : HeaderGasSnapshot) (layout : s.layout (GF := GF) base h old) :
    GasPricesOwned base prices s.values ⊢ (HeaderGasState base h old : IProp GF) := by
  iintro Hprices
  unfold HeaderGasState
  iright
  iexists prices, s.first, s.second, s.publishFirst, s.publishSecond
  isimp only [HeaderGasSnapshot.values] at Hprices
  iframe Hprices
  ipureintro
  exact layout

theorem headerGasLayout_success (s : HeaderGasSnapshot) (base : Loc) (h : Header) (old : HeaderValue)
    (second : Bool) (value : GasPriceValue) (layout : s.layout (GF := GF) base h old) :
    (s.update second value true).layout (GF := GF) base
      (headerSetGasPointer second h (arrayIndexRef GasPrice (sint.Z (headerGasSlot second)) base))
      (headerSetGasValue second old (some value)) := by
  rcases layout with ⟨hf, hs, vf, vs⟩
  cases second <;> simp_all [HeaderGasSnapshot.layout, HeaderGasSnapshot.update, headerSetGasPointer, headerSetGasValue, headerGasSlot]

/-- A failed read leaves the published pointer unchanged. If it was already
    published, its abstract contents reflect the partial update; otherwise the
    partial value remains private. -/
theorem headerGasLayout_failure (s : HeaderGasSnapshot) (base : Loc) (h : Header) (old : HeaderValue)
    (second : Bool) (value : GasPriceValue) (layout : s.layout (GF := GF) base h old) :
    (s.update second value (s.published second)).layout (GF := GF) base h
      (headerSetGasValue second old (if s.published second then some value else none)) := by
  rcases layout with ⟨hf, hs, vf, vs⟩
  cases second <;> simp_all [HeaderGasSnapshot.layout, HeaderGasSnapshot.update, HeaderGasSnapshot.published, headerSetGasValue]

/-- Null removes only the selected public pointer/value. The allocation and its
    private contents remain available for the other GasPrice field. -/
theorem headerGasState_null (base : Loc) (h : Header) (old : HeaderValue) (second : Bool) :
    HeaderGasState base h old ⊢
      (HeaderGasState base (headerSetGasPointer second h null) (headerSetGasValue second old none) : IProp GF) := by
  unfold HeaderGasState
  iintro Hstate
  icases Hstate with (Hnil | Hshared)
  · icases Hnil with %Hnil
    ileft
    ipureintro
    rcases Hnil with ⟨hb, hf, hs, vf, vs⟩
    cases second <;> simp_all [headerSetGasPointer, headerSetGasValue]
  · icases Hshared with ⟨%prices, %first, %last, %pf, %ps, Hprices, %Hlayout⟩
    rcases Hlayout with ⟨hf, hs, vf, vs⟩
    iright
    cases second
    · iexists prices, first, last, false, ps
      iframe Hprices
      ipureintro
      simp_all [headerSetGasPointer, headerSetGasValue]
    · iexists prices, first, last, pf, false
      iframe Hprices
      ipureintro
      simp_all [headerSetGasPointer, headerSetGasValue]

/-- Actual null/undefined call at either Header price field, at any cursor
    depth, with unchanged shared allocation and exact one-byte consumption. -/
theorem wp_headerGas_null (c dst shared base : Loc) (h : Header) (old : HeaderValue) (second : Bool)
    (v : cursor.Cursor) (xs rem : List w8) (head : w8) (dq : DFrac)
    (hn : head = W8 246 ∨ head = W8 247)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       dst ↦ h ∗ HeaderBody h old ∗ shared ↦ base ∗ HeaderGasState base h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c))
        (Val #(structFieldRef Header (headerGasName second) dst))) (Val #shared)) (Val #(headerGasSlot second)))
    {{ RET #interface.nil;
       dst ↦ headerSetGasPointer second h null ∗
       HeaderBody (headerSetGasPointer second h null) (headerSetGasValue second old none) ∗ shared ↦ base ∗
       HeaderGasState base (headerSetGasPointer second h null) (headerSetGasValue second old none) ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 ∧ sint.Z (v.pos' + W64 1) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨Hh, Hbody, Hshared, Hprices, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs [head] rem v.pos' v.buf'.len hp Hlen.1 hbytes
  simp only [List.length_singleton, Int.natCast_one] at Hnext
  have hi := input_suffix_head xs rem (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  icases headerGas_acc dst h second $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_gasPriceSlot_null c (structFieldRef Header (headerGasName second) dst) shared base
    (headerGasPointer second h) (headerGasSlot second) v xs head dq hp.1 hi hn
    $$ [Hfield Hshared Hc Hs] with ⟨Hfield, Hshared, Hc, Hs⟩
  · iframe Hfield Hshared Hc Hs
  ihave Hh := Hclose $$ %null Hfield
  ihave Hprices := headerGasState_null base h old second $$ Hprices
  iapply HΦ
  isimp only [headerBody_set_gas]
  iframe Hh Hbody Hshared Hprices Hc Hs
  ipureintro
  exact Hnext

/-- Cleanup from the private loop state also handles unpublished partial
    GasPrice slots. Ownership is discharged under the existing affine/GC model;
    the claim is reset of the public Header, not physical reclamation. -/
theorem wp_clearHeader_internal (dst base : Loc) (h : Header) (old : HeaderValue) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       dst ↦ h ∗ HeaderBody h old ∗ HeaderGasState base h old }}
      (App (Val (@! cborClearHeader)) (Val #dst))
    {{ RET #(); HeaderOwned dst headerZeroValue }} := by
  wp_start_folded as ⟨Hh, Hbody, Hprices⟩
  ihave Hchildren := headerGasState_publish base h old $$ [Hbody Hprices]
  · iframe Hbody Hprices
  wp_apply wp_clearHeader_owned dst h old $$ [Hh Hchildren] with Hzero
  · iframe Hh Hchildren
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasSlot_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasSnapshot_get
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasSnapshot_set
#print axioms Perennial.github_com.NethermindEth.juno.core.headerSetGasPointer_original
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGas_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.headerBody_set_gas
#print axioms Perennial.github_com.NethermindEth.juno.core.headerBody_set_gas_value
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasState_snapshot
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasLayout_success
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasLayout_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasState_null
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerGas_null
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_clearHeader_internal
