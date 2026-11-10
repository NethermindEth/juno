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

/-- A successful map read through either Header price field reuses the shared
    pair, publishes exactly the selected slot, and restores the loop invariant.
    The key-family restriction is inherited from the proved GasPrice map contract. -/
theorem wp_headerGas_existing_map (c dst shared base fields : Loc) (h : Header) (old : HeaderValue)
    (second : Bool) (prices : GoArray GasPrice 2) (snapshot : HeaderGasSnapshot) (g : GasPrice) (seen : w64)
    (v : cursor.Cursor) (f : cursor.Fields) (xs payload rem : List w8) (head : w8)
    (kind : GoInterface) (qf qo qn dq : DFrac) (d : Nat) (entries : List (GasPriceEntry (d + 1)))
    (layout : snapshot.layout (GF := GF) base h old) (unseen : HeaderGasUnseen seen snapshot)
    (hg : prices.arr[(sint.Z (headerGasSlot second)).toNat]? = some g)
    (enc : HeadArgument head payload (W64 entries.length)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32) (fresh : GasEntriesFresh (W64 0) entries)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ globalAddr cursor.ErrUnsupported ↦□ kind ∗
       dst ↦ h ∗ HeaderBody h old ∗ shared ↦ base ∗ GasPricesOwned base prices snapshot.values ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c))
        (Val #(structFieldRef Header (headerGasName second) dst))) (Val #shared)) (Val #(headerGasSlot second)))
    {{ RET #interface.nil;
       dst ↦ headerSetGasPointer second h (arrayIndexRef GasPrice (sint.Z (headerGasSlot second)) base) ∗
       HeaderBody (headerSetGasPointer second h (arrayIndexRef GasPrice (sint.Z (headerGasSlot second)) base))
         (headerSetGasValue second old (some (entries.foldl GasPriceEntry.apply (snapshot.value second)))) ∗
       shared ↦ base ∗
       HeaderGasReady base (headerSetGasPointer second h (arrayIndexRef GasPrice (sint.Z (headerGasSlot second)) base))
         (headerSetGasValue second old (some (entries.foldl GasPriceEntry.apply (snapshot.value second))))
         (headerKeySeen seen (some (headerGasKey second))) ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length)} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length)) =
           sint.Z v.pos' + 1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length)))
           (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨#Hglobal, #Hkind, Hh, Hbody, Hshared, Hprices, Hc, Hs, Hfields, Horder, Hnames⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hbytes' : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      ((head :: payload) ++ entries.flatMap GasPriceEntry.bytes) ++ rem := by
    simpa only [List.append_assoc] using hbytes
  have Hbounds := input_consumption xs ((head :: payload) ++ entries.flatMap GasPriceEntry.bytes)
    rem v.pos' v.buf'.len hp Hlen.1 hbytes'
  have hcount : (((head :: payload) ++ entries.flatMap GasPriceEntry.bytes).length : Int) =
      1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one]
    omega
  simp only [hcount] at Hbounds
  icases headerGas_acc dst h second $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_gasPriceSlot_existing_map_supported c (structFieldRef Header (headerGasName second) dst) shared
    (headerGasPointer second h) fields (headerGasSlot second) base prices snapshot.values g (snapshot.value second)
    v f xs payload rem head kind qf qo qn dq d entries (headerGasSlot_bound second) hg
    (headerGasSnapshot_get snapshot second) enc major hd hdepth fresh hp hbytes
    $$ [Hfield Hshared Hprices Hc Hs Hfields Horder Hnames]
    as ⟨%out, Hfield, Hshared, Hprices, Hc, Hs, Hfields, Horder, Hnames, %Hnext⟩
  · iframe Hglobal Hkind Hfield Hshared Hprices Hc Hs Hfields Horder Hnames
  ihave Hh := Hclose $$ %(arrayIndexRef GasPrice (sint.Z (headerGasSlot second)) base) Hfield
  isimp only [headerGasSnapshot_set snapshot second _ true] at Hprices
  ihave Hstate := headerGasReady_snapshot base _ _ _ (headerKeySeen seen (some (headerGasKey second)))
    (snapshot.update second (entries.foldl GasPriceEntry.apply (snapshot.value second)) true)
    (headerGasLayout_success (GF := GF) snapshot base h old second _ layout)
    (headerGasUnseen_write seen snapshot second _ true unseen) $$ Hprices
  iapply HΦ
  isimp only [headerBody_set_gas]
  iframe Hh Hbody Hshared Hstate Hc Hs Hfields Horder Hnames
  ipureintro
  exact ⟨Hnext.1, Hbounds.2.1, Hnext.2⟩

/-- With no previous allocation, the actual Header call allocates and zeroes
    both structs before decoding and publishing the selected one. -/
theorem wp_headerGas_new_map (c dst shared fields : Loc) (h : Header) (old : HeaderValue) (second : Bool) (seen : w64)
    (v : cursor.Cursor) (f : cursor.Fields) (xs payload rem : List w8) (head : w8)
    (kind : GoInterface) (qf qo qn dq : DFrac) (d : Nat) (entries : List (GasPriceEntry (d + 1)))
    (layout : headerGasZeroSnapshot.layout (GF := GF) null h old)
    (enc : HeadArgument head payload (W64 entries.length)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32) (fresh : GasEntriesFresh (W64 0) entries)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ globalAddr cursor.ErrUnsupported ↦□ kind ∗
       dst ↦ h ∗ HeaderBody h old ∗ shared ↦ null ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c))
        (Val #(structFieldRef Header (headerGasName second) dst))) (Val #shared)) (Val #(headerGasSlot second)))
    {{ (base : Loc), RET #interface.nil;
       dst ↦ headerSetGasPointer second h (arrayIndexRef GasPrice (sint.Z (headerGasSlot second)) base) ∗
       HeaderBody (headerSetGasPointer second h (arrayIndexRef GasPrice (sint.Z (headerGasSlot second)) base))
         (headerSetGasValue second old (some (entries.foldl GasPriceEntry.apply ⟨none, none⟩))) ∗
       shared ↦ base ∗
       HeaderGasReady base (headerSetGasPointer second h (arrayIndexRef GasPrice (sint.Z (headerGasSlot second)) base))
         (headerSetGasValue second old (some (entries.foldl GasPriceEntry.apply ⟨none, none⟩)))
         (headerKeySeen seen (some (headerGasKey second))) ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length)} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length)) =
           sint.Z v.pos' + 1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length)))
           (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨#Hglobal, #Hkind, Hh, Hbody, Hshared, Hc, Hs, Hfields, Horder, Hnames⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hbytes' : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      ((head :: payload) ++ entries.flatMap GasPriceEntry.bytes) ++ rem := by
    simpa only [List.append_assoc] using hbytes
  have Hbounds := input_consumption xs ((head :: payload) ++ entries.flatMap GasPriceEntry.bytes)
    rem v.pos' v.buf'.len hp Hlen.1 hbytes'
  have hcount : (((head :: payload) ++ entries.flatMap GasPriceEntry.bytes).length : Int) =
      1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one]
    omega
  simp only [hcount] at Hbounds
  icases headerGas_acc dst h second $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_gasPriceSlot_new_map_supported c (structFieldRef Header (headerGasName second) dst) shared
    (headerGasPointer second h) fields (headerGasSlot second)
    v f xs payload rem head kind qf qo qn dq d entries (headerGasSlot_bound second)
    enc major hd hdepth fresh hp hbytes
    $$ [Hfield Hshared Hc Hs Hfields Horder Hnames]
    as ⟨%base, %out, Hfield, Hshared, Hprices, Hc, Hs, Hfields, Horder, Hnames, %Hnext⟩
  · iframe Hglobal Hkind Hfield Hshared Hc Hs Hfields Horder Hnames
  ihave Hh := Hclose $$ %(arrayIndexRef GasPrice (sint.Z (headerGasSlot second)) base) Hfield
  have layout' : headerGasZeroSnapshot.layout (GF := GF) base h old := layout
  have hvalues := headerGasSnapshot_set headerGasZeroSnapshot second
    (entries.foldl GasPriceEntry.apply (GasPriceValue.mk none none)) true
  change (([⟨none, none⟩, ⟨none, none⟩] : List GasPriceValue).set
    (sint.Z (headerGasSlot second)).toNat (entries.foldl GasPriceEntry.apply ⟨none, none⟩)) =
    (headerGasZeroSnapshot.update second (entries.foldl GasPriceEntry.apply ⟨none, none⟩) true).values at hvalues
  isimp only [hvalues] at Hprices
  ihave Hstate := headerGasReady_snapshot base _ _ _ (headerKeySeen seen (some (headerGasKey second)))
    (headerGasZeroSnapshot.update second (entries.foldl GasPriceEntry.apply ⟨none, none⟩) true)
    (headerGasLayout_success (GF := GF) headerGasZeroSnapshot base h old second _ layout')
    (headerGasUnseen_write seen headerGasZeroSnapshot second _ true (headerGasUnseen_zero seen)) $$ Hprices
  iapply HΦ
  isimp only [headerBody_set_gas]
  iframe Hh Hbody Hshared Hstate Hc Hs Hfields Horder Hnames
  ipureintro
  exact ⟨Hnext.1, Hbounds.2.1, Hnext.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerGas_existing_map
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerGas_new_map
