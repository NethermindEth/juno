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

/-- The six nullable felt fields in Header's application schema. -/
inductive HeaderFeltField where
  | hash
  | parentHash
  | globalStateRoot
  | sequencerAddress
  | gasETH
  | gasSTRK

def headerFeltName : HeaderFeltField → GoString
  | .hash => go!"Hash"
  | .parentHash => go!"ParentHash"
  | .globalStateRoot => go!"GlobalStateRoot"
  | .sequencerAddress => go!"SequencerAddress"
  | .gasETH => go!"L1GasPriceETH"
  | .gasSTRK => go!"L1GasPriceSTRK"

def headerFeltPointer (field : HeaderFeltField) (h : Header) : Loc :=
  match field with
  | .hash => h.Hash'
  | .parentHash => h.ParentHash'
  | .globalStateRoot => h.GlobalStateRoot'
  | .sequencerAddress => h.SequencerAddress'
  | .gasETH => h.L1GasPriceETH'
  | .gasSTRK => h.L1GasPriceSTRK'

def headerSetFeltPointer (field : HeaderFeltField) (h : Header) (p : Loc) : Header :=
  match field with
  | .hash => {h with Hash' := p}
  | .parentHash => {h with ParentHash' := p}
  | .globalStateRoot => {h with GlobalStateRoot' := p}
  | .sequencerAddress => {h with SequencerAddress' := p}
  | .gasETH => {h with L1GasPriceETH' := p}
  | .gasSTRK => {h with L1GasPriceSTRK' := p}

def headerSetFeltValue (field : HeaderFeltField) (old : HeaderValue) (value : Option (GoArray w64 4)) : HeaderValue :=
  match field with
  | .hash => {old with hash := value}
  | .parentHash => {old with parentHash := value}
  | .globalStateRoot => {old with globalStateRoot := value}
  | .sequencerAddress => {old with sequencerAddress := value}
  | .gasETH => {old with l1GasPriceETH := value}
  | .gasSTRK => {old with l1GasPriceSTRK := value}

theorem headerSetFeltPointer_original (field : HeaderFeltField) (h : Header) :
    headerSetFeltPointer field h (headerFeltPointer field h) = h := by cases field <;> rfl

/-- Borrow any actual felt-pointer field and restore the whole Header after a
    write, preserving all fourteen other fields. -/
theorem headerFelt_acc (dst : Loc) (h : Header) (field : HeaderFeltField) :
    dst ↦ h ⊢ (iprop(
      structFieldRef Header (headerFeltName field) dst ↦ headerFeltPointer field h ∗
      (∀ p : Loc, structFieldRef Header (headerFeltName field) dst ↦ p -∗ dst ↦ headerSetFeltPointer field h p)) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup _ _ _ $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split _ _ _ $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, Hparent, Hnumber, Hroot, Hsequencer, Htransactions, Hevents,
    Htime, Hversion, Heth, Hsignatures, Hstrk, Hmode, HdataGas, Hl2Gas, -⟩
  cases field <;> isimp only [headerFeltName, headerFeltPointer, headerSetFeltPointer]
  all_goals
    iframe
    iintro %p Hvalue
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef]
    iframe

/-- Replacing one felt result preserves scalar values and every other child. -/
theorem headerBody_set_felt (h : Header) (old : HeaderValue) (field : HeaderFeltField)
    (p : Loc) (value : Option (GoArray w64 4)) :
    HeaderBody h old ∗ FeltResult p value ⊢
      (HeaderBody (headerSetFeltPointer field h p) (headerSetFeltValue field old value) : IProp GF) := by
  unfold HeaderBody
  iintro ⟨⟨%Hscalars, Hhash, Hparent, Hroot, Hsequencer, Heth, Hsignatures, Hstrk⟩, Hvalue⟩
  cases field <;> isimp only [headerSetFeltPointer, headerSetFeltValue]
  all_goals
    iframe
    ipureintro
    exact Hscalars

theorem headerChildren_set_felt (h : Header) (old : HeaderValue) (field : HeaderFeltField)
    (p : Loc) (value : Option (GoArray w64 4)) :
    HeaderChildren h old ∗ FeltResult p value ⊢
      (HeaderChildren (headerSetFeltPointer field h p) (headerSetFeltValue field old value) : IProp GF) := by
  unfold HeaderChildren
  iintro ⟨⟨Hbody, Hdata, Hl2⟩, Hvalue⟩
  ihave Hbody := headerBody_set_felt h old field p value $$ [Hbody Hvalue]
  · iframe Hbody Hvalue
  cases field <;> isimp only [headerSetFeltPointer, headerSetFeltValue] at Hbody
  all_goals
    isimp only [headerSetFeltPointer, headerSetFeltValue]
    iframe Hbody Hdata Hl2

/-- Felt writes do not affect the private shared-price allocation invariant. -/
theorem headerGasState_set_felt (base : Loc) (h : Header) (old : HeaderValue) (field : HeaderFeltField)
    (p : Loc) (value : Option (GoArray w64 4)) :
    HeaderGasState (GF := GF) base (headerSetFeltPointer field h p) (headerSetFeltValue field old value) =
      HeaderGasState base h old := by cases field <;> rfl

/-- Actual felt calls at each of Header's six felt fields, preserving
    the non-GasPrice children, unchanged input and exact suffix. Shared GasPrice
    ownership frames through headerGasState_set_felt. -/
theorem wp_headerFelt_valid (c dst : Loc) (h : Header) (old : HeaderValue) (field : HeaderFeltField)
    (v : cursor.Cursor) (xs wire rem : List w8) (value : Option (GoArray w64 4)) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32) (valid : FeltValid d wire value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       dst ↦ h ∗ HeaderBody h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef Header (headerFeltName field) dst)))
    {{ (p : Loc), RET #interface.nil; dst ↦ headerSetFeltPointer field h p ∗
       HeaderBody (headerSetFeltPointer field h p) (headerSetFeltValue field old value) ∗
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨Hh, Hchildren, Hc, Hs⟩
  icases headerFelt_acc dst h field $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readFelt_valid c (structFieldRef Header (headerFeltName field) dst) (headerFeltPointer field h)
    v xs wire rem value dq d hd hdepth valid hp hbytes $$ [Hfield Hc Hs] as %p ⟨Hfield, Hvalue, Hc, Hs, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Hh := Hclose $$ %p Hfield
  ihave Hchildren := headerBody_set_felt h old field p value $$ [Hchildren Hvalue]
  · iframe Hchildren Hvalue
  wp_end

/-- Any felt failure preserves all Header fields and prior child ownership,
    while returning the exact owned diagnostic and bounded offset. -/
theorem wp_headerFelt_failure (c dst : Loc) (h : Header) (old : HeaderValue) (field : HeaderFeltField)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason) (kind : GoInterface)
    (failure : FeltFailure d wire offset reason) (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ h ∗ HeaderBody h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef Header (headerFeltName field) dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ h ∗ HeaderBody h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       FeltFailureError err (v.pos' + W64 offset) reason kind ∗
       ⌜sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨#Hkind, Hh, Hchildren, Hc, Hs⟩
  icases headerFelt_acc dst h field $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readFelt_failure_bounded c (structFieldRef Header (headerFeltName field) dst) (headerFeltPointer field h)
    v xs wire dq d offset reason kind failure hd hdepth hp hbytes
    $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hoffset⟩
  · iframe Hkind Hfield Hc Hs
  ihave Hh := Hclose $$ %(headerFeltPointer field h) Hfield
  isimp only [headerSetFeltPointer_original] at Hh
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerSetFeltPointer_original
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFelt_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.headerBody_set_felt
#print axioms Perennial.github_com.NethermindEth.juno.core.headerChildren_set_felt
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasState_set_felt
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerFelt_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerFelt_failure
