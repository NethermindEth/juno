import ProofCheck.StructOpenSpec
import ProofCheck.StateMapOpeningSpec
import ProofCheck.InitializationErrors
import ProofCheck.StorageDiffsBoundary

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

/-- Every rejected StorageDiffs opening preserves the destination,
    cursor and input and returns the specified owned diagnostic. -/
theorem wp_readStorageDiffs_open_failure (c dst old : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (d : Nat) (reason : FeltFailureReason) (kind : GoInterface)
    (failure : StructOpenFailure d wire reason) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStorageDiffs)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ FeltFailureError err v.pos' reason kind }} := by
  cases failure with
  | outer hn hu failure =>
    simpa only [feltFailureAddr, FeltFailureError] using
      wp_readStorageDiffs_outer_failure (GF := GF) c dst old v xs wire dq _ kind hn hu failure hp hbytes
  | @count d head payload tail argument enc major short =>
    iintro %Φ ⟨#Hpkg, #Hkind, Hdst, Hc, Hs⟩ HΦ
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
    wp_apply wp_readStorageDiffs_count c dst old v xs payload tail head argument dq kind enc major hp.1 hbound
      (by rw [← htail]; omega) hi hafter $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    wp_end
  | @depth head payload tail argument enc major fit =>
    iintro %Φ ⟨#Hpkg, #Hkind, Hdst, Hc, Hs⟩ HΦ
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
    wp_apply wp_readStorageDiffs_depth c dst old v xs payload tail head argument dq kind enc major hp.1 (by omega) hbound
      (by rw [← htail]; omega) hi hafter $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    wp_end

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

/-- All independent map-opening rejections use the actual initialized diagnostic
    and preserve the old destination, before map allocation. -/
theorem wp_readStorageDiffs_open_rejected (c dst old : Loc)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d : Nat) (reason : StorageDiffsFailureReason)
    (failure : StorageDiffsSpec d wire (.failure 0 0 0 reason))
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStorageDiffs)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ StorageDiffsFailureOwned err v.pos' reason }} := by
  iintro %Φ ⟨#Hpkg, Hdst, Hc, Hs⟩ HΦ
  ihave Hglobals := cursor_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  ihave #Hkind := feltErrorGlobals_lookup kinds (storageDiffsDiagnostic reason) $$ Hglobals
  have opening := (storageDiffs_zero_depth_failure_open d 0 0 wire reason depth failure).2.2
  wp_apply wp_readStorageDiffs_open_failure c dst old v xs wire dq d
    (storageDiffsDiagnostic reason) (feltFailureKind (storageDiffsDiagnostic reason) kinds) opening hd hp hbytes
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  iapply HΦ
  iframe Hdst Hc Hs
  unfold StorageDiffsFailureOwned
  iexists kinds
  iframe Hglobals Herr

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readStorageDiffs_open_failure

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readStorageDiffs_open_rejected
