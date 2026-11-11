import ProofCheck.AddressSpec
import ProofCheck.AddressOwnership

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

local instance (dst : Loc) (low high : w64) :
    ⟦Slice l1.eth.Address.ty, (#dst, #low, #high)⟧ ⤳
      (if 0 ≤ sint.Z low ∧ sint.Z low ≤ sint.Z high ∧ sint.Z high ≤ 20 then
        #(slice.mk (arrayIndexRef w8 (sint.Z low) dst) (high - low) (W64 20 - low))
      else Panic "slice bounds out of range") := address_slice_step dst low high

/-- The translated typed address reader copies precisely the twenty wire bytes,
    or preserves its old array for null/undefined, with exact input consumption. -/
theorem wp_readAddress_valid (c dst : Loc) (old : GoArray w8 20) (v : cursor.Cursor)
    (xs item rest : List w8) (value : GoArray w8 20) (dq : DFrac) (valid : AddressValid old item value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadAddress)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ value ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs item rest v.pos' v.buf'.len hp Hlen.1 hbytes
  wp_auto
  cases valid with
  | @nullish head hn =>
    have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    wp_apply wp_null_at c v xs head dq hp.1 hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    wp_end
  | @bytes item body valid =>
    have hn : item.head? ≠ some (W8 246) ∧ item.head? ≠ some (W8 247) := by
      cases valid with
      | bytes enc major count length =>
        simp only [List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
        constructor <;> intro h <;> subst h <;> contradiction
    have hnWire : (item ++ rest).head? = item.head? := by
      cases item with
      | nil => have := bytesIntoValid_nonempty _ _ _ valid; simp at this
      | cons => rfl
    wp_apply wp_null_not_nullish c v xs (item ++ rest) dq (by rw [hnWire]; exact hn.1)
      (by rw [hnWire]; exact hn.2) hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    ihave %Hsize := array_len dst (DFrac.own 1) 20 old.arr $$ Hdst
    have size : old.arr.length = 20 := by omega
    ihave Hdst := (address_slice_owned dst old (DFrac.own 1)).1 $$ Hdst
    simp only [show sint.Z (W64 0) = 0 from rfl, go.arrayIndexRef_0, show W64 20 - W64 0 = W64 20 from rfl]
    wp_apply wp_bytesInto_valid c (slice.mk dst (W64 20) (W64 20)) old.arr v xs item body rest dq
      (by simpa only [size] using valid) hp hbytes $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs, %Hnext2⟩
    · iframe Hdst Hc Hs
    ihave Hdst := (address_slice_owned dst (array.mk 20 body) (DFrac.own 1)).2 $$ Hdst
    wp_end

/-- Rejected addresses preserve their complete prior array and cursor, and
    return the initialized byte-string diagnostic with owned formatting arguments. -/
theorem wp_readAddress_rejected (c dst : Loc) (old : GoArray w8 20) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (reason : BytesIntoFailureReason) (kinds : FeltErrorKinds)
    (failure : AddressRejected wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       FeltErrorGlobals kinds ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadAddress)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ BytesIntoError err v.pos' 20 kinds reason }} := by
  wp_start as ⟨Hglobals, Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_not_nullish c v xs wire dq failure.1 failure.2.1 hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  ihave %Hsize := array_len dst (DFrac.own 1) 20 old.arr $$ Hdst
  have size : old.arr.length = 20 := by omega
  ihave Hdst := (address_slice_owned dst old (DFrac.own 1)).1 $$ Hdst
  simp only [show sint.Z (W64 0) = 0 from rfl, go.arrayIndexRef_0, show W64 20 - W64 0 = W64 20 from rfl]
  wp_apply wp_bytesInto_rejected c (slice.mk dst (W64 20) (W64 20)) old.arr v xs wire dq reason kinds
    (by simpa only [size] using failure.2.2) hp hbytes $$ [Hdst Hc Hs Hglobals] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hglobals Hdst Hc Hs
  ihave Hdst := (address_slice_owned dst old (DFrac.own 1)).2 $$ Hdst
  isimp only [size] at Herr
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readAddress_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readAddress_rejected
