import ProofCheck.FeltMapFromBoundary
import ProofCheck.LimbsComplete
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

/-- The actual migrated-class reader parses its first named felt key, then
    rejects the named by-value felt. Null and undefined values are rejected too;
    the private map is not published and the old destination is preserved. -/
theorem wp_readMigratedClasses_first_value_failure (c dst old : Loc) (count : Nat) (v : cursor.Cursor) (xs payload keyItem wire : List w8)
    (head : w8) (key : FeltMapKey) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (fit : 2 * count ≤ (keyItem ++ wire).length)
    (countBound : count < 2^63)
    (valid : FeltMapKeyValid (d + 1) keyItem key)
    (failure : LimbsFailure (d + 1) wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ keyItem ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadMigratedClasses)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + keyItem.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ MigratedClassesFailureOwned err (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) (.element (.value reason)) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length)) = sint.Z v.pos' + 1 + payload.length + keyItem.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) = sint.Z v.pos' + 1 + payload.length + keyItem.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
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
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn.1 hn.2 $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_MapLen_encoded c v xs payload (keyItem ++ wire) head (W64 count) dq
    enc major hp.1 (by omega) htotal (by rw [← htail, castCount]; omega) hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply (wp_map_make2 (K := FeltMapKey) (V := FeltMapKey) (W64 count)) with %out Hmap
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
  wp_apply wp_limbs_failure c value_ptr (zero_val (GoArray w64 4)) {v with pos' := v.pos' + W64 (1 + payload.length) + W64 keyItem.length, depth' := v.depth' + W64 1} xs wire dq (d + 1) offset reason
    (feltFailureKind reason kinds) failure (by change sint.Z (v.depth' + W64 1) = (d + 1 : Nat); word) (by omega) keyPos HkeyNext.2.2
    $$ [value Hc Hs] as %err ⟨Hvalue, Hc, Hs, Herr⟩
  · iframe Hkind value Hc Hs
  have hpos : v.pos' + W64 (1 + payload.length) + W64 keyItem.length = v.pos' + W64 (1 + payload.length + keyItem.length) := by word
  have hoff : v.pos' + W64 (1 + payload.length) + W64 keyItem.length + W64 offset = v.pos' + W64 (1 + payload.length + keyItem.length + offset) := by word
  isimp only [hpos] at Hc
  isimp only [hoff] at Herr
  wp_for_post
  iapply HΦ
  iframe Hdst Hc Hs
  isplitl [Herr]
  · unfold MigratedClassesFailureOwned migratedClassesDiagnostic
    iexists kinds
    iframe Hglobals Herr
  ipureintro
  have failureBound := limbsFailure_offset _ _ _ _ failure
  have Hvalue := input_consumption xs wire [] _ v.buf'.len keyPos Hlen.1 (by simpa using HkeyNext.2.2)
  simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hopen
  constructor; · word
  constructor <;> word

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readMigratedClasses_first_value_failure
