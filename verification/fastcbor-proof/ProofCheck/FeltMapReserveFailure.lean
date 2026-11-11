import ProofCheck.FeltReserveComplete
import ProofCheck.FeltReserveWant
import ProofCheck.FeltMapFromBoundary
import ProofCheck.FeltMapOwnership
import ProofCheck.LimbsComplete
import ProofCheck.NullableFailure
import ProofCheck.StateMapDiagnostics
import ProofCheck.InitializationErrors

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

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

/-- The actual map reader parses its first key, consumes one shared reserve slot,
    then rejects a nonnull value. The unpublished map leaves the old destination
    untouched; the failed zero slot and the remaining reserve stay disjoint. -/
theorem wp_readFeltMapFrom_first_value_failure (c dst old spare : Loc) (reserve : GoSlice)
    (before count extra : Nat) (v : cursor.Cursor) (xs payload keyItem wire : List w8)
    (head : w8) (key : FeltMapKey) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (fit : 2 * count ≤ (keyItem ++ wire).length)
    (request : count + extra < 2^63)
    (valid : FeltMapKeyValid (d + 1) keyItem key)
    (failure : FeltFailure (d + 1) wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ keyItem ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       spare ↦ reserve ∗ FeltReserve reserve before ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (App (Val (@! cborReadFeltMapFrom)) (Val #c)) (Val #dst)) (Val #spare)) (Val #(W64 extra)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + keyItem.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ FeltMapFailureOwned err (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) (.element (.value reason)) ∗
       (∃ (remaining : GoSlice) (after : Nat) (temporary : Loc), spare ↦ remaining ∗ FeltReserve remaining after ∗
         temporary ↦ zero_val (GoArray w64 4) ∗ ⌜FeltReserveStep before after (W64 count + W64 extra)⌝) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length)) = sint.Z v.pos' + 1 + payload.length + keyItem.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) = sint.Z v.pos' + 1 + payload.length + keyItem.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start as ⟨Hspare, Hreserve, Hdst, Hc, Hs⟩
  ihave Hglobals := cursor_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  ihave #Hkind := feltErrorGlobals_lookup kinds reason $$ Hglobals
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hopen := input_consumption xs (head :: payload) (keyItem ++ wire) v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_assoc] using hbytes)
  have htotal : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hopen; omega
  have hlen := congrArg List.length hbytes
  simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
    List.length_append, List.length_cons] at hlen
  have htail : ((keyItem ++ wire).length : Int) = sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length) := by
    simp only [List.length_append, sint.nat, sint.Z] at Hlen hp hlen ⊢; omega
  have hi := input_suffix_head xs (payload ++ keyItem ++ wire) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hn : head ≠ W8 246 ∧ head ≠ W8 247 := by
    constructor <;> intro eq <;> subst head <;> contradiction
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ (keyItem ++ wire) := by
    have hstep : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
    rw [hstep]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  have castCount : uint.Z (W64 count) = (count : Int) := by word
  have hpositive : sint.Z (W64 count) > 0 := by word
  have hrequest : 0 < sint.Z (W64 count + W64 extra) := by word
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn.1 hn.2 $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_MapLen_encoded c v xs payload (keyItem ++ wire) head (W64 count) dq
    enc major hp.1 (by omega) htotal (by rw [← htail, castCount]; omega) hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply (wp_map_make2 (K := FeltMapKey) (V := Loc) (W64 count)) with %out Hmap
  wp_for
  simp only [show sint.Z (W64 0) < sint.Z (W64 count) from by word, _root_.decide_true, ite_true]
  wp_auto
  have hopenPos : 0 ≤ sint.Z (v.pos' + W64 (1 + payload.length)) ∧
      sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hopen; word
  have hopenBytes : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = keyItem ++ wire := by
    simpa only [List.length_cons, Nat.add_comm payload.length 1, Int.natCast_add, Int.natCast_one] using Hopen.2.2
  have Hlimbs := wp_limbs_valid (GF := GF) c key_ptr (zero_val (GoArray w64 4)) key
    {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1}
    xs keyItem wire dq (d + 1) (by change sint.Z (v.depth' + W64 1) = (d + 1 : Nat); word) (by omega) valid hopenPos hopenBytes
  wp_apply Hlimbs
    $$ [key Hc Hs] with ⟨Hkey, Hc, Hs, %HkeyNext⟩
  · iframe key Hc Hs
  have keyPos : 0 ≤ sint.Z (v.pos' + W64 (1 + payload.length) + W64 keyItem.length) ∧
      sint.Z (v.pos' + W64 (1 + payload.length) + W64 keyItem.length) ≤ sint.Z v.buf'.len := by
    simp only at HkeyNext; constructor; · rw [HkeyNext.1]; omega
    exact HkeyNext.2.1
  have excluded := feltFailure_not_nullish (d + 1) offset wire reason failure
  wp_apply wp_null_not_nullish c {v with pos' := v.pos' + W64 (1 + payload.length) + W64 keyItem.length, depth' := v.depth' + W64 1} xs wire dq excluded.1 excluded.2 keyPos HkeyNext.2.2 $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_nextFelt_complete spare reserve before (W64 count + W64 extra) hrequest
    $$ [Hspare Hreserve] as %temporary ⟨%remaining, %after, Htemporary, Hspare, Hreserve, %Hstep⟩
  · iframe Hspare Hreserve
  wp_apply wp_limbs_felt_failure c temporary (zero_val (GoArray w64 4)) {v with pos' := v.pos' + W64 (1 + payload.length) + W64 keyItem.length, depth' := v.depth' + W64 1} xs wire dq (d + 1) offset reason
    (feltFailureKind reason kinds) failure (by change sint.Z (v.depth' + W64 1) = (d + 1 : Nat); word) (by omega) keyPos HkeyNext.2.2
    $$ [Htemporary Hc Hs] as %err ⟨Htemporary, Hc, Hs, Herr⟩
  · iframe Hkind Htemporary Hc Hs
  have hpos : v.pos' + W64 (1 + payload.length) + W64 keyItem.length = v.pos' + W64 (1 + payload.length + keyItem.length) := by word
  have hoff : v.pos' + W64 (1 + payload.length) + W64 keyItem.length + W64 offset = v.pos' + W64 (1 + payload.length + keyItem.length + offset) := by word
  isimp only [hpos] at Hc
  isimp only [hoff] at Herr
  wp_for_post
  iapply HΦ
  iframe Hdst Hc Hs
  isplitl [Herr]
  · unfold FeltMapFailureOwned feltMapDiagnostic
    iexists kinds
    iframe Hglobals Herr
  isplitl [Hspare Hreserve Htemporary]
  · iexists remaining, after, temporary
    iframe Hspare Hreserve Htemporary
    ipureintro; exact Hstep
  ipureintro
  have failureBound := feltFailure_offset _ _ _ _ failure
  have Hvalue := input_consumption xs wire [] _ v.buf'.len keyPos Hlen.1 (by simpa using HkeyNext.2.2)
  simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hopen
  constructor; · word
  constructor <;> word

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltMapFrom_first_value_failure
