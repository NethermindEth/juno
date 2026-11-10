import ProofCheck.LimbsReject

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

/-- In-place Limbs always expects an array, including for null/undefined. -/
theorem wp_limbs_outer_failure (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (failure : ExpectedFailure (W8 128) wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 128) reason kind }} := by
  cases wire with
  | nil =>
    cases failure
    wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_nil] at hlen
    have heof : v.pos' = v.buf'.len := by simp only [sint.nat] at Hlen hlen; word
    isimp only [expectedFailureAddr] at Hkind
    isimp only [ExpectedFailureError] at HΦ
    wp_apply wp_limbs_eof c dst old v xs dq kind heof $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    wp_end
  | cons head tail =>
    exact wp_limbs_expect_reject c dst old v xs tail head dq
      iprop(expectedFailureAddr (GF := GF) reason ↦□ kind)
      (fun err => ExpectedFailureError err v.pos' (W8 128) reason kind)
      (by simpa using expectedFailure_not_four (head :: tail) reason failure) hp hbytes
      (wp_expect_failure c v xs (head :: tail) (W8 128) dq reason kind (by decide) failure hp hbytes)

/-- Failed retained limbs preserve the caller's array through all actual shortcuts. -/
theorem wp_limbs_limb_failure (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs payload wire : List w8) (head : w8) (count : w64) (items : List UnsignedWire)
    (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (short : items.length < 4) (more : items.length < uint.Z count)
    (failure : ExpectedFailure (W8 0) wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len) (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       ExpectedFailureError err (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)) (W8 0) reason kind }} := by
  wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  let pos := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)
  have Hnext := input_consumption xs ((head :: payload) ++ unsignedBytes items) wire v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpnext : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    dsimp only [pos]; constructor <;> word
  have hsuffix : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = wire := by
    simpa only [pos, List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm, Int.add_left_comm, Int.add_assoc] using Hnext.2.2
  wp_apply wp_limbs_prefix_reject c dst old v xs payload wire head count items dq
    iprop(expectedFailureAddr (GF := GF) reason ↦□ kind)
    (fun err => ExpectedFailureError err pos (W8 0) reason kind)
    enc hm (expectedFailure_unsignedReject wire reason failure) short more hp hdepth hbytes
    (wp_uint64_failure c {v with pos' := pos, depth' := v.depth' + W64 1} xs wire dq reason kind failure hpnext hsuffix)
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end

/-- Every specified recursive Skip failure inside ignored felt contents retains
    its exact diagnostic while restoring the whole cursor and the caller's four-word array. -/
theorem wp_limbs_extras_failure (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs payload wire : List w8) (head : w8) (count : w64)
    (items : List UnsignedWire) (extras : List (List w8)) (dq : DFrac)
    (reason : SkipFailureReason) (kind : GoInterface)
    (d offset : Nat) (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hcount : items.length = 4) (hmore : 4 + extras.length < uint.Z count)
    (valid : ∀ item ∈ extras, SkipValid (d + 1) item) (failure : SkipFailure (d + 1) wire offset reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ extras.flatten ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       skipFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       SkipFailureError err (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length) + W64 offset) reason kind }} := by
  wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  let pos := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length)
  have Hnext := input_consumption xs ((head :: payload) ++ unsignedBytes items ++ extras.flatten)
    wire v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpnext : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    dsimp only [pos]; constructor <;> word
  have hsuffix : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = wire := by
    simpa only [pos, List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm, Int.add_left_comm, Int.add_assoc] using Hnext.2.2
  have hbad :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         skipFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
      {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
         c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
         SkipFailureError err (pos + W64 offset) reason kind }} := by
    wp_start_folded as ⟨#Hkind, Hc, Hs⟩
    wp_apply wp_Skip_failure_bounded c {v with pos' := pos, depth' := v.depth' + W64 1} xs wire dq
      (d + 1) offset reason kind failure (by word) (by omega) hpnext hsuffix $$ [Hc Hs] as %err ⟨Hc, Hs, Herr, %Hoff⟩
    · iframe Hkind Hc Hs
    wp_end
  wp_apply wp_limbs_extras_reject c dst old v xs payload wire head count items extras dq
    iprop(skipFailureAddr (GF := GF) reason ↦□ kind)
    (fun err => SkipFailureError err (pos + W64 offset) reason kind)
    d hd hdepth enc hm hcount hmore valid hp hbytes
    hbad
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end

/-- Every independently specified felt failure is realized by the translated
    production in-place reader. All actual leaf/recursive contracts are discharged. -/
theorem wp_limbs_felt_failure (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason) (kind : GoInterface)
    (failure : FeltFailure d wire offset reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ FeltFailureError err (v.pos' + W64 offset) reason kind }} := by
  cases failure with
  | outer hn hu failure =>
    have hz : v.pos' + W64 0 = v.pos' := by word
    simpa only [feltFailureAddr, FeltFailureError, Int.natCast_zero, hz] using
      wp_limbs_outer_failure (GF := GF) c dst old v xs _ dq _ kind failure hp hbytes
  | @limb d head payload tail count items reason enc major bound short more failure =>
    simpa only [feltFailureAddr, FeltFailureError, Int.natCast_add, Int.natCast_one] using
      wp_limbs_limb_failure (GF := GF) c dst old v xs payload tail head count items dq reason kind
        enc major short more failure hp (by omega) hbytes
  | @extras d offset head payload tail count items extras reason enc major bound four more valid failure =>
    have hpos : v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length + offset : Nat) =
        v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length) + W64 offset := by word
    simpa only [feltFailureAddr, FeltFailureError, hpos] using
      wp_limbs_extras_failure (GF := GF) c dst old v xs payload tail head count items extras dq reason kind
        d offset hd bound enc major four more valid failure hp hbytes
  | @depth head payload tail count enc major =>
    wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have hi := input_suffix_head xs (payload ++ tail) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    have Hnext := input_consumption xs (head :: payload) tail v.pos' v.buf'.len hp Hlen.1 hbytes
    have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
      simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
      omega
    have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ tail := by
      have hn : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
      rw [hn]
      exact input_suffix_after xs [head] (payload ++ tail) _ _ hbytes
    have hz : v.pos' + W64 0 = v.pos' := by word
    isimp only [feltFailureAddr] at Hkind
    isimp only [FeltFailureError, SkipFailureError, Int.natCast_zero, hz] at HΦ
    wp_apply wp_limbs_depth c dst old v xs payload tail head count dq kind enc major hp.1 (by omega)
      hbound hi hafter $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_outer_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_limb_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_extras_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_felt_failure
