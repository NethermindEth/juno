import ProofCheck.StructOpenSpec

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

/-- Every rejected record opening returns a closed zero-count state, an
    unchanged cursor/input and the specified owned diagnostic. -/
theorem wp_openStruct_failure (c fields : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (d : Nat) (reason : FeltFailureReason) (kind : GoInterface)
    (failure : StructOpenFailure d wire reason) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"OpenStruct")) (Val #fields))
    {{ (err : Loc), RET (PairV #({(zero_val cursor.Struct) with fields' := fields})
        #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ FeltFailureError err v.pos' reason kind }} := by
  cases failure with
  | outer hn hu failure =>
    simpa only [feltFailureAddr, FeltFailureError] using
      wp_openStruct_expected_failure (GF := GF) c fields v xs wire dq _ kind hn hu failure hp hbytes
  | @count d head payload tail argument enc major short =>
    iintro %Φ ⟨#Hpkg, #Hkind, Hc, Hs⟩ HΦ
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have Hnext := input_consumption xs (head :: payload) tail v.pos' v.buf'.len hp Hlen.1 hbytes
    have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
      simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
      List.length_append, List.length_cons] at hlen
    have htail : (tail.length : Int) = sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length) := by
      simp only [sint.nat, sint.Z] at Hlen hp hlen ⊢; omega
    have hi := input_suffix_head xs (payload ++ tail) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ tail := by
      have hn : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
      rw [hn]
      exact input_suffix_after xs [head] (payload ++ tail) _ _ hbytes
    isimp only [feltFailureAddr, skipFailureAddr] at Hkind
    isimp only [FeltFailureError, SkipFailureError] at HΦ
    wp_apply wp_openStruct_count c fields v xs payload tail head argument dq kind enc major hp.1 hbound
      (by rw [← htail]; omega) hi hafter $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    wp_end
  | @depth head payload tail argument enc major fit =>
    iintro %Φ ⟨#Hpkg, #Hkind, Hc, Hs⟩ HΦ
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have Hnext := input_consumption xs (head :: payload) tail v.pos' v.buf'.len hp Hlen.1 hbytes
    have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
      simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
      List.length_append, List.length_cons] at hlen
    have htail : (tail.length : Int) = sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length) := by
      simp only [sint.nat, sint.Z] at Hlen hp hlen ⊢; omega
    have hi := input_suffix_head xs (payload ++ tail) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ tail := by
      have hn : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
      rw [hn]
      exact input_suffix_after xs [head] (payload ++ tail) _ _ hbytes
    isimp only [feltFailureAddr] at Hkind
    isimp only [FeltFailureError, SkipFailureError] at HΦ
    wp_apply wp_openStruct_depth c fields v xs payload tail head argument dq kind enc major hp.1 (by omega) hbound
      (by rw [← htail]; omega) hi hafter $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    wp_end

/-- Opening a supported definite map consumes exactly its head and enters one
    nesting level. Entry bytes remain untouched and unread. -/
theorem wp_openStruct_map (c fields : Loc) (v : cursor.Cursor) (xs payload body : List w8)
    (head : w8) (count : w64) (dq : DFrac) (d : Nat)
    (enc : HeadArgument head payload count) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32) (fit : 2 * uint.Z count ≤ body.length)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"OpenStruct")) (Val #fields))
    {{ RET (PairV #(cursor.Struct.mk fields count (W64 0) (W64 0) true) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = body ∧
         sint.Z (v.depth' + W64 1) = d + 1⌝ }} := by
  iintro %Φ ⟨#Hpkg, Hc, Hs⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs (head :: payload) body v.pos' v.buf'.len hp Hlen.1 hbytes
  have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
  have hlen := congrArg List.length hbytes
  simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
    List.length_append, List.length_cons] at hlen
  have htail : (body.length : Int) = sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length) := by
    simp only [sint.nat, sint.Z] at Hlen hp hlen ⊢; omega
  have hi := input_suffix_head xs (payload ++ body) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ body := by
    have hn : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
    rw [hn]
    exact input_suffix_after xs [head] (payload ++ body) _ _ hbytes
  wp_apply wp_openStruct_encoded c fields v xs payload body head count dq enc major hp.1 (by omega) hbound
    (by rw [← htail]; exact fit) hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  iapply HΦ
  iframe Hc Hs
  ipureintro
  simp only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm (payload.length : Int) 1] at Hnext
  exact ⟨by omega, Hnext.2.1, Hnext.2.2, by word⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_openStruct_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_openStruct_map
