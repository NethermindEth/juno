import ProofCheck.SignaturesComplete

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

/-- Application value of the two nullable felt amounts, independent of pointers. -/
structure GasPriceValue where
  wei : Option (GoArray w64 4)
  fri : Option (GoArray w64 4)

def GasPriceChildren (g : GasPrice) (value : GasPriceValue) : IProp GF :=
  iprop(FeltResult g.PriceInWei' value.wei ∗ FeltResult g.PriceInFri' value.fri)

def GasPriceOwned (p : Loc) (value : GasPriceValue) : IProp GF :=
  iprop(∃ g : GasPrice, p ↦ g ∗ GasPriceChildren g value)

/-- Shared storage owns the two struct slots once; published Header pointers
    reference those slots rather than owning overlapping copies of the structs. -/
def GasPricesOwned (base : Loc) (prices : GoArray GasPrice 2) (values : List GasPriceValue) : IProp GF :=
  iprop(base ↦ prices ∗ [∗list] g;value ∈ prices.arr;values, GasPriceChildren g value)

def gasPriceFieldName (wei : Bool) : GoString := if wei then go!"PriceInWei" else go!"PriceInFri"
def gasPriceField (wei : Bool) (g : GasPrice) : Loc := if wei then g.PriceInWei' else g.PriceInFri'
def gasPriceSet (wei : Bool) (g : GasPrice) (p : Loc) : GasPrice :=
  if wei then {g with PriceInWei' := p} else {g with PriceInFri' := p}

theorem gasPriceSet_original (wei : Bool) (g : GasPrice) :
    gasPriceSet wei g (gasPriceField wei g) = g := by cases wei <;> rfl

/-- Borrow either actual struct field, then reassemble the whole struct after
    writing any pointer. The other field remains owned and unchanged. -/
theorem gasPrice_field_acc (dst : Loc) (g : GasPrice) (wei : Bool) :
    dst ↦ g ⊢ (iprop(
      structFieldRef GasPrice (gasPriceFieldName wei) dst ↦ gasPriceField wei g ∗
      (∀ p : Loc, structFieldRef GasPrice (gasPriceFieldName wei) dst ↦ p -∗ dst ↦ gasPriceSet wei g p)) : IProp GF) := by
  iintro Hg
  icases typedPointsto_not_null_dup _ _ _ $$ Hg with ⟨Hg, %Hnn⟩
  icases typedPointsto_split _ _ _ $$ Hg with Hg
  isimp only [TypedPointsto.typedPointstoDef] at Hg
  icases Hg with ⟨Hwei, Hfri, -⟩
  cases wei <;> isimp only [gasPriceFieldName, gasPriceField, gasPriceSet, Bool.false_eq_true, ite_false, ite_true]
  · iframe Hfri
    iintro %p Hfri
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef]
    iframe Hwei Hfri
  · iframe Hwei
    iintro %p Hwei
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef]
    iframe Hwei Hfri

/-- The actual felt call used by either GasPrice dispatch branch updates exactly
    that struct field and returns owned felt contents with the exact suffix. -/
theorem wp_gasPrice_field_valid (c dst : Loc) (g : GasPrice) (wei : Bool) (v : cursor.Cursor)
    (xs wire rem : List w8) (value : Option (GoArray w64 4)) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32) (valid : FeltValid d wire value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ g ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef GasPrice (gasPriceFieldName wei) dst)))
    {{ (p : Loc), RET #interface.nil; dst ↦ gasPriceSet wei g p ∗ FeltResult p value ∗
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨Hg, Hc, Hs⟩
  icases gasPrice_field_acc dst g wei $$ Hg with ⟨Hfield, Hclose⟩
  wp_apply wp_readFelt_valid c (structFieldRef GasPrice (gasPriceFieldName wei) dst) (gasPriceField wei g)
    v xs wire rem value dq d hd hdepth valid hp hbytes $$ [Hfield Hc Hs] as %p ⟨Hfield, Hvalue, Hc, Hs, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Hg := Hclose $$ %p Hfield
  wp_end

/-- A failed felt call preserves both GasPrice pointer fields, including a
    previous successful amount, and returns the owned exact inner diagnostic. -/
theorem wp_gasPrice_field_failure (c dst : Loc) (g : GasPrice) (wei : Bool) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason) (kind : GoInterface)
    (failure : FeltFailure d wire offset reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ g ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef GasPrice (gasPriceFieldName wei) dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ g ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ FeltFailureError err (v.pos' + W64 offset) reason kind ∗
       ⌜sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨#Hkind, Hg, Hc, Hs⟩
  icases gasPrice_field_acc dst g wei $$ Hg with ⟨Hfield, Hclose⟩
  wp_apply wp_readFelt_failure_bounded c (structFieldRef GasPrice (gasPriceFieldName wei) dst) (gasPriceField wei g)
    v xs wire dq d offset reason kind failure hd hdepth hp hbytes
    $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hoffset⟩
  · iframe Hkind Hfield Hc Hs
  ihave Hg := Hclose $$ %(gasPriceField wei g) Hfield
  isimp only [gasPriceSet_original] at Hg
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPriceSet_original
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPrice_field_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_gasPrice_field_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_gasPrice_field_failure
