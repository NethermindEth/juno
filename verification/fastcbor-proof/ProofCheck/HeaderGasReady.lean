import ProofCheck.HeaderSeen

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

/-- Updating only the key bookkeeping preserves ownership and the zero
    storage guarantee for every field that remains unseen. -/
theorem headerGasReady_mark (base : Loc) (h : Header) (old : HeaderValue) (seen : w64) (choice : Option HeaderField) :
    HeaderGasReady base h old seen ⊢ (HeaderGasReady base h old (headerKeySeen seen choice) : IProp GF) := by
  unfold HeaderGasReady
  iintro Hready
  icases Hready with (Hnil | Hshared)
  · ileft
    iexact Hnil
  · icases Hshared with ⟨%prices, %snapshot, Hprices, %Hlayout⟩
    iright
    iexists prices, snapshot
    iframe Hprices
    ipureintro
    exact ⟨Hlayout.1, headerGasUnseen_mark seen snapshot choice Hlayout.2⟩

/-- Null clears only the selected publication flag and public value, while
    retaining the other unseen slot's zero state. -/
theorem headerGasReady_null (base : Loc) (h : Header) (old : HeaderValue) (seen : w64) (second : Bool) :
    HeaderGasReady base h old seen ⊢
      (HeaderGasReady base (headerSetGasPointer second h null) (headerSetGasValue second old none)
        (headerKeySeen seen (some (headerGasKey second))) : IProp GF) := by
  unfold HeaderGasReady
  iintro Hready
  icases Hready with (Hnil | Hshared)
  · icases Hnil with %Hnil
    ileft
    ipureintro
    rcases Hnil with ⟨hb, hf, hs, vf, vs⟩
    cases second <;> simp_all [headerSetGasPointer, headerSetGasValue]
  · icases Hshared with ⟨%prices, %snapshot, Hprices, %Hlayout⟩
    iright
    iexists prices, (snapshot.update second (snapshot.value second) false)
    have hvalues : (snapshot.update second (snapshot.value second) false).values = snapshot.values := by
      cases second <;> rfl
    isimp only [hvalues]
    iframe Hprices
    ipureintro
    constructor
    · rcases Hlayout.1 with ⟨hf, hs, vf, vs⟩
      cases second <;> simp_all [HeaderGasSnapshot.layout, HeaderGasSnapshot.update,
        headerSetGasPointer, headerSetGasValue]
    · exact headerGasUnseen_write seen snapshot second (snapshot.value second) false Hlayout.2

/-- Actual null/undefined call at either Header price field, at any cursor
    depth, with unchanged shared allocation and exact one-byte consumption. -/
theorem wp_headerGas_null_ready (c dst shared base : Loc) (h : Header) (old : HeaderValue) (seen : w64) (second : Bool)
    (v : cursor.Cursor) (xs rem : List w8) (head : w8) (dq : DFrac)
    (hn : head = W8 246 ∨ head = W8 247)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       dst ↦ h ∗ HeaderBody h old ∗ shared ↦ base ∗ HeaderGasReady base h old seen ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c))
        (Val #(structFieldRef Header (headerGasName second) dst))) (Val #shared)) (Val #(headerGasSlot second)))
    {{ RET #interface.nil;
       dst ↦ headerSetGasPointer second h null ∗
       HeaderBody (headerSetGasPointer second h null) (headerSetGasValue second old none) ∗ shared ↦ base ∗
       HeaderGasReady base (headerSetGasPointer second h null) (headerSetGasValue second old none)
         (headerKeySeen seen (some (headerGasKey second))) ∗
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
  ihave Hprices := headerGasReady_null base h old seen second $$ Hprices
  iapply HΦ
  isimp only [headerBody_set_gas]
  iframe Hh Hbody Hshared Hprices Hc Hs
  ipureintro
  exact Hnext

/-- The stronger loop state still discharges actual Header cleanup, including
    unpublished partial values, without any extra heap assumptions. -/
theorem wp_clearHeader_ready (dst base : Loc) (h : Header) (old : HeaderValue) (seen : w64) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       dst ↦ h ∗ HeaderBody h old ∗ HeaderGasReady base h old seen }}
      (App (Val (@! cborClearHeader)) (Val #dst))
    {{ RET #(); HeaderOwned dst headerZeroValue }} := by
  wp_start_folded as ⟨Hh, Hbody, Hready⟩
  ihave Hstate := headerGasReady_forget base h old seen $$ Hready
  wp_apply wp_clearHeader_internal dst base h old $$ [Hh Hbody Hstate] with Hzero
  · iframe Hh Hbody Hstate
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasReady_mark
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasReady_null
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerGas_null_ready
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_clearHeader_ready
