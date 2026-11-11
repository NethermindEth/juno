import ProofCheck.FeltMapFromFirstKeyFailure
import ProofCheck.LimbsComplete

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

/-- The actual outer storage map parses its first key, then propagates the
    first inner key error. Neither private map is published. Both map heads
    remain entered and every byte and diagnostic offset is accounted for. -/
theorem wp_readStorageDiffs_first_inner_key_failure (c dst old : Loc) (count innerCount : Nat)
    (v : cursor.Cursor) (xs payload keyItem wire innerPayload tail : List w8)
    (head innerHead : w8) (key : FeltMapKey) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (countBound : count < 2^63) (fit : 2 * count ≤ (keyItem ++ wire).length)
    (valid : FeltMapKeyValid (d + 1) keyItem key)
    (innerEnc : HeadArgument innerHead innerPayload (W64 innerCount))
    (innerMajor : innerHead &&& W8 224 = W8 160) (innerPositive : 0 < innerCount)
    (innerFit : 2 * innerCount ≤ tail.length)
    (split : wire = (innerHead :: innerPayload) ++ tail)
    (failure : LimbsFailure (d + 1 + 1) tail offset reason)
    (hd : sint.Z v.depth' = d) (depth : d + 1 < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ keyItem ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStorageDiffs)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + keyItem.length + 1 + innerPayload.length), depth' := v.depth' + W64 2} ∗
       v.buf' ↦*{dq} xs ∗ StorageDiffsFailureOwned err (v.pos' + W64 (1 + payload.length + keyItem.length + 1 + innerPayload.length + offset)) (.element (.value (.element (.key reason)))) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + 1 + innerPayload.length)) = sint.Z v.pos' + 1 + payload.length + keyItem.length + 1 + innerPayload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + 1 + innerPayload.length + offset)) = sint.Z v.pos' + 1 + payload.length + keyItem.length + 1 + innerPayload.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + 1 + innerPayload.length + offset)) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
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
  have hsize : xs.length < 2^63 := by
    have machine : sint.Z v.buf'.len < 2^63 := by word
    simp only [sint.nat, sint.Z] at Hlen machine
    omega
  have innerFitWhole : 2 * innerCount ≤ xs.length := by
    have splitLength := congrArg List.length split
    simp only [List.length_append, List.length_cons] at splitLength innerFit hlen
    simp only [sint.nat, sint.Z] at Hlen hp hlen ⊢
    omega
  have innerBound : innerCount < 2^63 := by omega
  have hint : W64 count - W64 1 = W64 (count - 1 : Nat) := by word
  simp only [hint]
  wp_apply wp_readFeltMapFrom_first_key_failure c value_ptr null spare_ptr slice.nil (W64 (count - 1 : Nat)) innerCount
    {v with pos' := v.pos' + W64 (1 + payload.length) + W64 keyItem.length, depth' := v.depth' + W64 1}
    xs innerPayload tail innerHead dq (d + 1) offset reason innerEnc innerMajor
    innerPositive innerBound innerFit failure
    (by change sint.Z (v.depth' + W64 1) = (d + 1 : Nat); word) depth keyPos
    (by simpa only [split] using HkeyNext.2.2)
    $$ [value spare Hc Hs] as %err ⟨Hspare, Hvalue, Hc, Hs, Herr, %Hinner⟩
  · iframe value spare Hc Hs
  have hpos : v.pos' + W64 (1 + payload.length) + W64 keyItem.length + W64 (1 + innerPayload.length) =
      v.pos' + W64 (1 + payload.length + keyItem.length + 1 + innerPayload.length) := by word
  have hoff : v.pos' + W64 (1 + payload.length) + W64 keyItem.length + W64 (1 + innerPayload.length + offset) =
      v.pos' + W64 (1 + payload.length + keyItem.length + 1 + innerPayload.length + offset) := by word
  have hdepth : v.depth' + W64 1 + W64 1 = v.depth' + W64 2 := by word
  isimp only [hpos, hdepth] at Hc
  isimp only [hoff] at Herr
  wp_for_post
  iapply HΦ
  iframe Hdst Hc Hs
  isplitl [Herr]
  · isimp only [FeltMapFailureOwned] at Herr
    unfold StorageDiffsFailureOwned storageDiffsDiagnostic
    iassumption
  ipureintro
  simp only [List.length_cons, Nat.add_comm payload.length 1, Int.natCast_add, Int.natCast_one] at Hopen
  simp only at Hinner HkeyNext
  rw [hpos, hoff] at Hinner
  constructor
  · rw [Hinner.1, HkeyNext.1]; omega
  constructor
  · rw [Hinner.2.1, HkeyNext.1]; omega
  · exact Hinner.2.2

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readStorageDiffs_first_inner_key_failure
