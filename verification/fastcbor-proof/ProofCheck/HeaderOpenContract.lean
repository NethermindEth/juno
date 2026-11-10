import ProofCheck.HeaderOpen
import ProofCheck.StructOpenComplete

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

/-- Null and undefined preserve all existing Header values and child ownership
    at arbitrary cursor offsets/depths, consuming exactly one byte. -/
theorem wp_readHeader_nullish_owned (c dst fields : Loc) (h : Header) (value : HeaderValue)
    (v : cursor.Cursor) (xs rest : List w8) (head : w8) (dq : DFrac)
    (hn : head = W8 246 ∨ head = W8 247)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ dst ↦ h ∗ HeaderChildren h value ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadHeader)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ h ∗ HeaderChildren h value ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 ∧ sint.Z (v.pos' + W64 1) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start_folded as ⟨#Hfields, Hh, Hchildren, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have Hnext := input_consumption xs [head] rest v.pos' v.buf'.len hp Hlen.1 hbytes
  simp only [List.length_singleton, Int.natCast_one] at Hnext
  wp_apply wp_readHeader_nullish c dst fields h v xs head dq hp.1 hi hn $$ [Hh Hc Hs] with ⟨Hh, Hc, Hs⟩
  · iframe Hfields Hh Hc Hs
  wp_end

/-- Every opening failure at a public entry is propagated after actual cleanup,
    with a zero owned Header and unchanged input. This includes arbitrary malformed
    or incomplete heads, type/tag errors, and impossible pair counts. -/
theorem wp_decodeHeaderFirst_open_failure (dst fields : Loc) (input : GoSlice) (wire : List w8)
    (dq : DFrac) (reason : FeltFailureReason) (kind : GoInterface)
    (failure : StructOpenFailure 0 wire reason) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ feltFailureAddr (GF := GF) reason ↦□ kind ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       HeaderOwned dst headerZeroValue ∗ input ↦*{dq} wire ∗ FeltFailureError err (W64 0) reason kind }} := by
  wp_start as ⟨#Hfields, #Hkind, Hdst, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hp : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z input.len := ⟨by decide, Hlen.2⟩
  have hb : subslice (sint.nat (W64 0)) (sint.nat input.len) wire = wire := by
    simp only [show sint.nat (W64 0) = 0 by rfl, subslice, List.drop_zero, ← Hlen.1, List.take_length]
  wp_auto
  wp_apply wp_zeroHeader dst $$ [Hdst] with Hdst
  · iframe Hdst
  wp_apply wp_newCursor input
  wp_apply wp_readHeader_open_failure _ dst fields (zero_val Header) {(zero_val cursor.Cursor) with buf' := input}
    wire wire dq 0 reason kind failure rfl hp hb $$ [c Hdst Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hfields Hkind c Hdst Hs
  wp_apply wp_clearHeader dst (zero_val Header) $$ [Hdst] with Hdst
  · iframe Hdst
  ieval (change (FeltFailureError err (W64 0) reason kind : IProp GF) ⊢ _) at Herr
  iapply HΦ
  iframe Hs Herr
  unfold HeaderOwned
  iexists (zero_val Header)
  iframe Hdst
  iapply headerChildren_zero

/-- Full-input decoding propagates the same opening error and owned zero result;
    trailing-data handling does not run after the parse failure. -/
theorem wp_decodeHeader_open_failure (dst fields : Loc) (input : GoSlice) (wire : List w8)
    (dq : DFrac) (reason : FeltFailureReason) (kind : GoInterface)
    (failure : StructOpenFailure 0 wire reason) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ feltFailureAddr (GF := GF) reason ↦□ kind ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       HeaderOwned dst headerZeroValue ∗ input ↦*{dq} wire ∗ FeltFailureError err (W64 0) reason kind }} := by
  wp_start as ⟨#Hfields, #Hkind, Hdst, Hs⟩
  wp_auto
  wp_apply wp_decodeHeaderFirst_open_failure dst fields input wire dq reason kind failure $$ [Hdst Hs]
    as %err ⟨Hdst, Hs, Herr⟩
  · iframe Hfields Hkind Hdst Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readHeader_nullish_owned
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_open_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_open_failure
