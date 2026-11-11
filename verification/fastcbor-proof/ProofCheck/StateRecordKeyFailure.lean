import ProofCheck.StateDiffErrors
import ProofCheck.StateUpdateErrors
import ProofCheck.StateDiffKeyRead
import ProofCheck.StateUpdateKeyRead

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

/-- The actual record loop returns a first-key rejection without touching any old
    field or child. All supported positive map-head widths are covered. -/
theorem wp_readStateDiff_first_key_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs payload wire : List w8) (head : w8) (dq : DFrac) (d count : Nat) (reason : KeyFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (fit : 2 * count ≤ wire.length) (failure : KeyRejected wire reason)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStateDiff)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ StateDiffFailureOwned err (v.pos' + W64 (1 + payload.length)) (.key reason) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = wire ∧
         StateDiffRecordRejected d old ((head :: payload) ++ wire) old
           (1 + payload.length) (1 + payload.length) 1 (.key reason)⌝ }} := by
  iintro %Φ ⟨#Hpkg, Howned, Hc, Hs⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ wire) [] v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_nil] using hbytes)
  have htotal : sint.Z v.pos' + 1 + payload.length + wire.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    omega
  have hfit : (count : Int) < 2^63 := by word
  have Hfit : 2 * uint.Z (W64 count) ≤ wire.length := by word
  have hleft : 0 < sint.Z (W64 count) := by word
  have size : uint.Z (W64 count) = (count : Int) := by word
  have prefixFailure : StateDiffPairsRejected (d + 1) count [] old wire old 0 0 0 (.key reason) := by
    have hcount : count = (count - 1) + 1 := by omega
    rw [hcount]
    exact .key failure
  have spec : StateDiffRecordRejected d old ((head :: payload) ++ wire) old
      (1 + payload.length) (1 + payload.length) 1 (.key reason) := by
    simpa only [Nat.add_zero] using StateDiffRecordRejected.map enc major size depth fit prefixFailure
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply cursor_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  ihave #Hkind := keyErrorGlobals_lookup kinds reason $$ Hglobals
  ihave #Htable := state_initialized_table (GF := GF) .stateDiff $$ Hpkg
  isimp only [StateTable, stateGlobal, StaticFieldsShared] at Htable
  icases Htable with ⟨%fields, #Hglobal, %f, #Hfields, #Horder, #Hnames, %Hmask⟩
  wp_func_call
  wp_call
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_map c fields v xs payload wire head (W64 count) dq d enc major hd depth Hfit hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hopen⟩
  · iframe Hc Hs
  have hopenPos : 0 ≤ sint.Z (v.pos' + W64 (1 + payload.length)) ∧
      sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len := by word
  have hopenBytes : subslice (sint.nat (v.pos' + W64 (1 + payload.length)))
      (sint.nat v.buf'.len) xs = wire := Hopen.2.2.1
  wp_for
  wp_apply wp_nextStateDiffKey_rejected validator c s_ptr fields
    {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1}
    (cursor.Struct.mk fields (W64 count) (W64 0) (W64 0) true) f xs wire
    DFrac.discard DFrac.discard dq reason (keyFailureKind reason kinds) rfl hleft
    (by change 0 ≤ (0 : Int) ∧ (0 : Nat) ≤ _; omega) failure hopenPos hopenBytes
    $$ [Hc s Hs] as %err ⟨Hc, Hst, _, _, Hs, Herr⟩
  · iframe Hkind Hc s Hfields Horder Hs
  wp_for_post
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Herr]
  · unfold StateDiffFailureOwned StateDiffFailureError
    iexists kinds
    iframe Hglobals Herr
  ipureintro
  exact ⟨Hopen.1, Hopen.2.1, hopenBytes, spec⟩

/-- The actual record loop returns a first-key rejection without touching any old
    field or child. All supported positive map-head widths are covered. -/
theorem wp_readStateUpdate_first_key_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : StateUpdateValue) (v : cursor.Cursor)
    (xs payload wire : List w8) (head : w8) (dq : DFrac) (d count : Nat) (reason : KeyFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (fit : 2 * count ≤ wire.length) (failure : KeyRejected wire reason)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       StateUpdateOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStateUpdate)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateUpdateOwned dst old ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ StateUpdateFailureOwned err (v.pos' + W64 (1 + payload.length)) (.key reason) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = wire ∧
         StateUpdateRecordRejected d old ((head :: payload) ++ wire) old
           (1 + payload.length) (1 + payload.length) 1 (.key reason)⌝ }} := by
  iintro %Φ ⟨#Hpkg, Howned, Hc, Hs⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ wire) [] v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_nil] using hbytes)
  have htotal : sint.Z v.pos' + 1 + payload.length + wire.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    omega
  have hfit : (count : Int) < 2^63 := by word
  have Hfit : 2 * uint.Z (W64 count) ≤ wire.length := by word
  have hleft : 0 < sint.Z (W64 count) := by word
  have size : uint.Z (W64 count) = (count : Int) := by word
  have prefixFailure : StateUpdatePairsRejected (d + 1) count [] old wire old 0 0 0 (.key reason) := by
    have hcount : count = (count - 1) + 1 := by omega
    rw [hcount]
    exact .key failure
  have spec : StateUpdateRecordRejected d old ((head :: payload) ++ wire) old
      (1 + payload.length) (1 + payload.length) 1 (.key reason) := by
    simpa only [Nat.add_zero] using StateUpdateRecordRejected.map enc major size depth fit prefixFailure
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply cursor_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  ihave #Hkind := keyErrorGlobals_lookup kinds reason $$ Hglobals
  ihave #Htable := state_initialized_table (GF := GF) .stateUpdate $$ Hpkg
  isimp only [StateTable, stateGlobal, StaticFieldsShared] at Htable
  icases Htable with ⟨%fields, #Hglobal, %f, #Hfields, #Horder, #Hnames, %Hmask⟩
  wp_func_call
  wp_call
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_map c fields v xs payload wire head (W64 count) dq d enc major hd depth Hfit hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hopen⟩
  · iframe Hc Hs
  have hopenPos : 0 ≤ sint.Z (v.pos' + W64 (1 + payload.length)) ∧
      sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len := by word
  have hopenBytes : subslice (sint.nat (v.pos' + W64 (1 + payload.length)))
      (sint.nat v.buf'.len) xs = wire := Hopen.2.2.1
  wp_for
  wp_apply wp_nextStateUpdateKey_rejected validator c s_ptr fields
    {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1}
    (cursor.Struct.mk fields (W64 count) (W64 0) (W64 0) true) f xs wire
    DFrac.discard DFrac.discard dq reason (keyFailureKind reason kinds) rfl hleft
    (by change 0 ≤ (0 : Int) ∧ (0 : Nat) ≤ _; omega) failure hopenPos hopenBytes
    $$ [Hc s Hs] as %err ⟨Hc, Hst, _, _, Hs, Herr⟩
  · iframe Hkind Hc s Hfields Horder Hs
  wp_for_post
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Herr]
  · unfold StateUpdateFailureOwned StateUpdateFailureError
    iexists kinds
    iframe Hglobals Herr
  ipureintro
  exact ⟨Hopen.1, Hopen.2.1, hopenBytes, spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readStateDiff_first_key_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readStateUpdate_first_key_rejected
