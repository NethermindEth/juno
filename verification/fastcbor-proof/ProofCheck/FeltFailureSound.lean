import ProofCheck.FeltClassify

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

/-- All outer expectation failures compose through the actual typed reader,
    after excluding its earlier null/undefined success branch. -/
theorem wp_readFelt_outer_failure (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (failure : ExpectedFailure (W8 128) wire reason)
    (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 128) reason kind }} := by
  cases failure with
  | head tail fault =>
    simpa only [expectedFailureAddr, ExpectedFailureError] using
      wp_readFelt_head_fault (GF := GF) c dst old v xs tail _ dq _ kind fault hp hbytes
  | incomplete tail info sizeEq short =>
    simpa only [expectedFailureAddr, ExpectedFailureError] using
      wp_readFelt_head_truncated (GF := GF) c dst old v xs tail _ _ dq kind info sizeEq short hp hbytes
  | typeMismatch tail enc major notTag =>
    simpa only [expectedFailureAddr, ExpectedFailureError] using
      wp_readFelt_head_type (GF := GF) c dst old v xs _ tail _ _ dq kind enc major notTag
        (by simpa using hn) (by simpa using hu) hp hbytes
  | tag tail enc major isTag =>
    simpa only [expectedFailureAddr, ExpectedFailureError] using
      wp_readFelt_head_tag (GF := GF) c dst old v xs _ tail _ _ dq kind enc isTag hp hbytes
  | eof =>
    wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_nil] at hlen
    have heof : v.pos' = v.buf'.len := by simp only [sint.nat] at Hlen hlen; word
    isimp only [expectedFailureAddr] at Hkind
    isimp only [ExpectedFailureError] at HΦ
    wp_apply wp_readFelt_eof c dst old v xs dq kind heof $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    wp_end

/-- All unsigned-limb expectation failures compose through every felt head
    width and both shortcuts, preserving the original output pointer. -/
theorem wp_readFelt_limb_failure (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload wire : List w8) (head : w8) (count : w64) (items : List UnsignedWire)
    (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (short : items.length < 4) (more : items.length < uint.Z count)
    (failure : ExpectedFailure (W8 0) wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len) (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       ExpectedFailureError err (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)) (W8 0) reason kind }} := by
  cases failure with
  | head tail fault =>
    simpa only [expectedFailureAddr, ExpectedFailureError] using
      wp_readFelt_limb_fault (GF := GF) c dst old v xs payload tail head _ count items dq _ kind
        enc hm short more hp hdepth hbytes fault
  | incomplete tail info sizeEq truncated =>
    simpa only [expectedFailureAddr, ExpectedFailureError] using
      wp_readFelt_limb_truncated (GF := GF) c dst old v xs payload tail head _ count _ items dq kind
        enc hm short more hp hdepth hbytes info sizeEq truncated
  | typeMismatch tail badenc major notTag =>
    simpa only [expectedFailureAddr, ExpectedFailureError] using
      wp_readFelt_limb_type (GF := GF) c dst old v xs payload _ tail head _ count _ items dq kind
        enc hm short more hp hdepth hbytes badenc major notTag
  | tag tail badenc major isTag =>
    simpa only [expectedFailureAddr, ExpectedFailureError] using
      wp_readFelt_limb_tag (GF := GF) c dst old v xs payload _ tail head _ count _ items dq kind
        enc hm short more hp hdepth hbytes badenc isTag
  | eof =>
    wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
      List.length_append, List.length_cons, List.length_nil] at hlen
    have heof : v.pos' + W64 (1 + payload.length + (unsignedBytes items).length) = v.buf'.len := by
      simp only [sint.nat] at Hlen hlen; word
    isimp only [expectedFailureAddr] at Hkind
    isimp only [ExpectedFailureError, heof] at HΦ
    wp_apply wp_readFelt_limb_eof c dst old v xs payload head count items dq kind enc hm short more hp hdepth
      (by simpa only [List.append_nil] using hbytes) $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    wp_end

/-- Every independently specified felt failure is realized by the translated
    production reader. All actual leaf/recursive contracts are discharged. -/
theorem wp_readFelt_failure (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason) (kind : GoInterface)
    (failure : FeltFailure d wire offset reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ FeltFailureError err (v.pos' + W64 offset) reason kind }} := by
  cases failure with
  | outer hn hu failure =>
    have hz : v.pos' + W64 0 = v.pos' := by word
    simpa only [feltFailureAddr, FeltFailureError, Int.natCast_zero, hz] using
      wp_readFelt_outer_failure (GF := GF) c dst old v xs _ dq _ kind failure hn hu hp hbytes
  | @limb d head payload tail count items reason enc major bound short more failure =>
    simpa only [feltFailureAddr, FeltFailureError, Int.natCast_add, Int.natCast_one] using
      wp_readFelt_limb_failure (GF := GF) c dst old v xs payload tail head count items dq reason kind
        enc major short more failure hp (by omega) hbytes
  | @extras d offset head payload tail count items extras reason enc major bound four more valid failure =>
    have hpos : v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length + offset : Nat) =
        v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length) + W64 offset := by word
    simpa only [feltFailureAddr, FeltFailureError, hpos] using
      wp_readFelt_extras_failure (GF := GF) c dst old v xs payload tail head count items extras dq reason kind
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
    wp_apply wp_readFelt_depth c dst old v xs payload tail head count dq kind enc major hp.1 (by omega)
      hbound hi hafter $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    wp_end

/-- Failure offsets are exact signed positions in the original buffer, including
    EOF. Cursor rollback never discards the owned inner diagnostic. -/
theorem wp_readFelt_failure_bounded (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason) (kind : GoInterface)
    (failure : FeltFailure d wire offset reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ FeltFailureError err (v.pos' + W64 offset) reason kind ∗
       ⌜sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hoff := feltFailure_offset d offset wire reason failure
  have Hnext := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_nil] using hbytes)
  have hpos : sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len := by constructor <;> word
  wp_apply wp_readFelt_failure c dst old v xs wire dq d offset reason kind failure hd hdepth hp hbytes
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_outer_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_limb_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_failure_bounded
