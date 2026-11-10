import ProofCheck.KeyText
import ProofCheck.FieldKeySpec
import ProofCheck.KeyFailureSound

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Every complete key rejection is already in the production-proved legacy
    family or is an arbitrary malformed UTF-8 payload. -/
theorem keyRejected_existing_or_utf8 (wire : List w8) (reason : KeyFailureReason)
    (failure : KeyRejected wire reason) :
    KeyFailure wire reason ∨ ∃ (head : w8) (payload body rem : List w8) (argument : w64),
      reason = .invalidUTF8 ∧ wire = (head :: payload) ++ body ++ rem ∧
      HeadArgument head payload argument ∧ head &&& W8 224 = W8 96 ∧
      uint.Z argument = body.length ∧ ¬ Utf8Valid body := by
  cases failure with
  | head tail fault => exact Or.inl (.head tail fault)
  | incomplete tail info sizeEq short => exact Or.inl (.incomplete tail info sizeEq short)
  | eof => exact Or.inl .eof
  | integerRange enc major bound => exact Or.inl (.integerRange enc major bound)
  | tag enc major => exact Or.inl (.tag enc major)
  | typeMismatch enc text uint negative tag => exact Or.inl (.typeMismatch enc text uint negative tag)
  | textTruncated enc major short => exact Or.inl (.textTruncated enc major short)
  | invalidUTF8 enc major count invalid => exact Or.inr ⟨_, _, _, _, _, rfl, rfl, enc, major, count, invalid⟩

variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Public field selection for arbitrary valid text keys and every head width.
    All lookup/parser helper WPs are discharged except the explicit validator. -/
theorem wp_nextField_text_valid (validator : Utf8ValidatorContract (GF := GF))
    (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (names : List GoString) (xs payload key rem : List w8) (head : w8)
    (kind : GoInterface) (qf qo qn dq : DFrac)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ []) (hsorted : FieldHeadsOrdered order)
    (hapart : FieldEncodingsApart order) (hcache : CachedShortTable order)
    (htable : InlineFieldTable order names) (hnames : names.length ≤ 64)
    (enc : HeadArgument head payload (W64 key.length)) (hm : head &&& W8 224 = W8 96)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ key ++ rem)
    (valid : Utf8Valid key) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (code : w64) (err : GoInterface), RET (PairV #code #err); ∃ next : w64,
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + key.length)) = sint.Z v.pos' + 1 + payload.length + key.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + key.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + key.length))) (sint.nat v.buf'.len) xs = rem⌝ ∗
       ⌜FieldSearchAdvance order ((head :: payload) ++ key ++ rem) s.next' next⌝ ∗
       FieldReadOutcome st {s with left' := s.left' - W64 1, next' := next} names key code err kind
         (v.pos' + W64 (1 + payload.length + key.length)) }} := by
  exact wp_nextField_text c st fields v s f order names xs payload key rem head kind qf qo qn dq
    hfields hleft hstart hnonempty hsorted hapart hcache htable hnames enc hm hp hbytes
    (wp_Key_text validator c v xs payload key rem head dq enc hm valid hp hbytes)

/-- Every complete key rejection propagates through the actual public field
    reader, conditional only on the explicit production validator contract. -/
theorem wp_nextField_rejected_utf8 (validator : Utf8ValidatorContract (GF := GF))
    (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (names : List GoString) (xs wire : List w8)
    (qf qo dq : DFrac) (reason : KeyFailureReason) (kind : GoInterface)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ []) (hsorted : FieldHeadsOrdered order)
    (hcache : CachedShortTable order) (htable : InlineFieldTable order names)
    (hascii : ∀ name ∈ names, ASCIIBytes name)
    (failure : KeyRejected wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       keyFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs ∗ KeyFailureError err v.pos' reason kind }} := by
  rcases keyRejected_existing_or_utf8 wire reason failure with existing | fullUtf8
  · exact wp_nextField_failure c st fields v s f order names xs wire qf qo dq reason kind
      hfields hleft hstart hnonempty hsorted hcache htable hascii existing hp hbytes
  · obtain ⟨head, payload, body, rem, argument, reasonEq, wireEq, enc, major, count, invalid⟩ := fullUtf8
    subst reason
    rw [wireEq] at hbytes failure
    have argumentEq : argument = W64 body.length := by word
    subst argument
    have rejected := wp_Key_invalid_utf8 validator c v xs payload body rem head dq kind enc major invalid hp hbytes
    exact wp_nextField_keyRejected c st fields v v s f order names xs _ .invalidUTF8 qf qo dq
      iprop(globalAddr cursor.ErrSyntax ↦□ kind)
      (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.pos' (go!"key is not valid UTF-8") slice.nil))
      hfields hleft hstart hnonempty hsorted hcache htable hascii failure hp hbytes rejected

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.keyRejected_existing_or_utf8
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_text_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_rejected_utf8
