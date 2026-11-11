import ProofCheck.KeyRejected
import ProofCheck.IntsComplete

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

theorem wp_peek_prefix (c : Loc) (v : cursor.Cursor) (xs tail : List w8) (head : w8) (dq : DFrac)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: tail) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Peek")) (Val #()))
    {{ RET (PairV #head #interface.nil); c ↦ v ∗ v.buf' ↦*{dq} xs }} := by
  wp_start_folded as ⟨Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hi := input_suffix_head xs tail (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have Hnext := input_consumption xs [head] tail v.pos' v.buf'.len hp Hlen.1 hbytes
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by
    simp only [List.length_singleton, Int.natCast_one] at Hnext; omega
  wp_apply wp_peek_at c v xs head dq ⟨hp.1, hlt⟩ hi $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

/-- Bounded unsigned keys are returned with their actual numeric value. -/
theorem wp_readIntegerKey_unsigned (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (enc : HeadArgument head payload argument)
    (major : head &&& W8 224 = W8 0) (bound : uint.Z argument < 2 ^ 63)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (@! cborReadIntegerKey)) (Val #c))
    {{ RET (PairV (PairV #argument #true) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (head :: payload).length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (head :: payload).length) = sint.Z v.pos' + (head :: payload).length ∧
         sint.Z (v.pos' + W64 (head :: payload).length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (head :: payload).length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hc, Hs⟩
  wp_auto
  wp_apply wp_peek_prefix c v xs (payload ++ rem) head dq hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [major]
  wp_auto
  wp_apply wp_uint64_wire c v xs rem ⟨head, payload, argument, major, enc⟩ dq hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
  · iframe Hc Hs
  have hgood : ¬ uint.Z (W64 9223372036854775807) < uint.Z argument := by word
  simp only [hgood, _root_.decide_false]
  wp_auto
  iapply HΦ
  isimp only [UnsignedWire.bytes, List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm (payload.length : Int) 1] at Hc
  try isimp only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm (payload.length : Int) 1]
  iframe Hc Hs
  ipureintro
  simpa only [UnsignedWire.bytes, List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm (payload.length : Int) 1, Int.add_assoc] using Hnext

/-- Unsigned overflow consumes its complete head and returns the bare sentinel. -/
theorem wp_readIntegerKey_overflow (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 0)
    (bound : 2 ^ 63 ≤ uint.Z argument)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr cursor.ErrType ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (@! cborReadIntegerKey)) (Val #c))
    {{ RET (PairV (PairV #(W64 0) #false) #kind);
       c ↦ {v with pos' := v.pos' + W64 (head :: payload).length} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_peek_prefix c v xs (payload ++ rem) head dq hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [major]
  wp_auto
  wp_apply wp_uint64_wire c v xs rem ⟨head, payload, argument, major, enc⟩ dq hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
  · iframe Hc Hs
  have hbad : uint.Z (W64 9223372036854775807) < uint.Z argument := by word
  simp only [hbad, _root_.decide_true]
  wp_auto
  wp_end

/-- Negative integer keys are consumed but treated as unknown fields. -/
theorem wp_readIntegerKey_negative (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (enc : HeadArgument head payload argument)
    (major : head &&& W8 224 = W8 32) (bound : uint.Z argument < 2 ^ 63)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (@! cborReadIntegerKey)) (Val #c))
    {{ RET (PairV (PairV #(W64 0) #false) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hc, Hs⟩
  wp_auto
  wp_apply wp_peek_prefix c v xs (payload ++ rem) head dq hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [major]
  wp_auto
  wp_apply wp_Key_integer c v xs payload rem head argument dq enc (Or.inr major) bound hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
  · iframe Hc Hs
  iapply HΦ
  iframe Hc Hs
  ipureintro; exact Hnext

/-- Valid text keys are consumed and treated as unknown. The actual UTF-8
    validator remains the same explicit unresolved premise as for Header. -/
theorem wp_readIntegerKey_text (validator : Utf8ValidatorContract (GF := GF))
    (c : Loc) (v : cursor.Cursor) (xs payload key rem : List w8) (head : w8) (dq : DFrac)
    (enc : HeadArgument head payload (W64 key.length)) (major : head &&& W8 224 = W8 96)
    (valid : Utf8Valid key)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ key ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (@! cborReadIntegerKey)) (Val #c))
    {{ RET (PairV (PairV #(W64 0) #false) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + key.length)) = sint.Z v.pos' + 1 + payload.length + key.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + key.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + key.length))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hc, Hs⟩
  wp_auto
  wp_apply wp_peek_prefix c v xs (payload ++ key ++ rem) head dq hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [major]
  wp_auto
  wp_apply wp_Key_text validator c v xs payload key rem head dq enc major valid hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
  · iframe Hc Hs
  iapply HΦ
  iframe Hc Hs
  ipureintro; exact Hnext

/-- Malformed unsigned heads preserve the original Uint64 error and rollback. -/
theorem wp_readIntegerKey_unsigned_failure (c : Loc) (v : cursor.Cursor) (xs tail : List w8)
    (head : w8) (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (major : head &&& W8 224 = W8 0) (failure : ExpectedFailure (W8 0) (head :: tail) reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: tail) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (@! cborReadIntegerKey)) (Val #c))
    {{ (err : Loc), RET (PairV (PairV #(W64 0) #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 0) reason kind }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_peek_prefix c v xs tail head dq hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [major]
  wp_auto
  wp_apply wp_uint64_failure c v xs (head :: tail) dq reason kind failure hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Non-unsigned keys use the complete Key rejection contract. -/
theorem wp_readIntegerKey_other_failure (validator : Utf8ValidatorContract (GF := GF))
    (c : Loc) (v : cursor.Cursor) (xs tail : List w8) (head : w8) (dq : DFrac)
    (reason : KeyFailureReason) (kind : GoInterface)
    (major : head &&& W8 224 ≠ W8 0) (failure : KeyRejected (head :: tail) reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: tail) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       keyFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (@! cborReadIntegerKey)) (Val #c))
    {{ (err : Loc), RET (PairV (PairV #(W64 0) #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ KeyFailureError err v.pos' reason kind }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_peek_prefix c v xs tail head dq hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [major, _root_.decide_false]
  wp_auto
  wp_apply wp_Key_rejected validator c v xs (head :: tail) dq reason kind failure hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- EOF first allocates a Peek error, then returns the Key error with no progress. -/
theorem wp_readIntegerKey_eof (c : Loc) (v : cursor.Cursor) (xs : List w8) (dq : DFrac)
    (kind : GoInterface) (heof : v.pos' = v.buf'.len) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (@! cborReadIntegerKey)) (Val #c))
    {{ (err : Loc), RET (PairV (PairV #(W64 0) #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  wp_auto
  wp_apply wp_peek_eof c v kind heof $$ [Hc] as %discarded ⟨Hc, Hdiscarded⟩
  · iframe Hkind Hc
  wp_apply wp_key_eof c v kind heof ⟨Hwf.1, Hwf.2⟩ $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hkind Hc
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_peek_prefix
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readIntegerKey_unsigned
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readIntegerKey_overflow
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readIntegerKey_negative
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readIntegerKey_text
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readIntegerKey_unsigned_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readIntegerKey_other_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readIntegerKey_eof
