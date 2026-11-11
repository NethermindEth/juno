import ProofCheck.WantedSelected
import ProofCheck.WantedSeen
import ProofCheck.SkipFailureSound

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

/-- Unknown integer keys are consumed before Skip; a rejected value retains
    that consumption, the current pair count and the original container depth. -/
theorem wp_nextWanted_integer_skip_rejected (c st : Loc) (v : cursor.Cursor) (s : cursor.Struct)
    (xs payload wire : List w8) (head : w8) (argument : w64) (dq : DFrac)
    (d offset : Nat) (reason : SkipFailureReason) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (major : head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32) (bound : uint.Z argument < 2^63)
    (failure : SkipFailure d wire offset reason) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (left : 0 < sint.Z s.left')
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       skipFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextWanted")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ st ↦ s ∗ v.buf' ↦*{dq} xs ∗
       SkipFailureError err (v.pos' + W64 (1 + payload.length) + W64 offset) reason kind ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length) + W64 offset) = sint.Z v.pos' + 1 + payload.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length) + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start as ⟨#Hkind, Hc, Hst, Hs⟩
  wp_auto
  wp_for
  have hz : sint.Z (W64 0) = 0 := rfl
  simp only [hz, left, _root_.decide_true, ite_true]
  wp_auto
  wp_apply wp_Key_integer c v xs payload wire head argument dq enc major bound hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hkey⟩
  · iframe Hc Hs
  wp_apply wp_Skip_failure_bounded c {v with pos' := v.pos' + W64 (1 + payload.length)} xs wire dq d offset reason kind
    failure hd depth ⟨by rw [Hkey.1]; omega, Hkey.2.1⟩ Hkey.2.2
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr, %Herror⟩
  · iframe Hkind Hc Hs
  wp_for_post
  iapply HΦ
  iframe Hc Hst Hs Herr
  ipureintro
  exact ⟨Hkey.1, by dsimp only at Herror; omega, Herror.2⟩

/-- Every ignored text-key path (unknown name, duplicate or all-seen shortcut)
    propagates a Skip failure with exact retained state and diagnostic. -/
theorem wp_nextWanted_text_skip_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (names : List GoString) (xs payload key wire : List w8) (head : w8) (qf qn dq : DFrac)
    (d offset : Nat) (reason : SkipFailureReason) (kind : GoInterface)
    (hfields : s.fields' = fields) (left : 0 < sint.Z s.left')
    (ignored : ∀ field, FieldMatchResult names key field true → s.seen' &&& (W64 1 <<< field) ≠ W64 0)
    (enc : HeadArgument head payload (W64 key.length)) (major : head &&& W8 224 = W8 96) (valid : Utf8Valid key)
    (failure : SkipFailure d wire offset reason) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ key ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       skipFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗
       fields ↦{qf} f ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextWanted")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} ∗ st ↦ s ∗
       fields ↦{qf} f ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs ∗
       SkipFailureError err (v.pos' + W64 (1 + payload.length + key.length) + W64 offset) reason kind ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + key.length)) = sint.Z v.pos' + 1 + payload.length + key.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + key.length) + W64 offset) = sint.Z v.pos' + 1 + payload.length + key.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + key.length) + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start as ⟨#Hkind, Hc, Hst, Hfields, Hnames, Hs⟩
  wp_auto
  wp_for
  have hz : sint.Z (W64 0) = 0 := rfl
  simp only [hz, left, _root_.decide_true, ite_true]
  wp_auto
  wp_apply wp_Key_text validator c v xs payload key wire head dq enc major valid hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hkey⟩
  · iframe Hc Hs
  simp only [hfields]
  icases (cursor.Fields.Fields_access_load_all fields f qf).access_strict $$ Hfields with ⟨Hall, Hrestore⟩
  wp_auto
  by_cases allSeen : s.seen' = f.all'
  · simp only [allSeen, _root_.decide_true, Bool.not_true, ite_false]
    wp_auto
    ihave Hfields := Hrestore $$ Hall
    wp_apply wp_Skip_failure_bounded c {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} xs wire dq d offset reason kind
      failure hd depth ⟨by rw [Hkey.1]; omega, Hkey.2.1⟩ Hkey.2.2
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr, %Herror⟩
    · iframe Hkind Hc Hs
    wp_for_post
    iapply HΦ
    iframe Hc Hst Hfields Hnames Hs Herr
    ipureintro
    exact ⟨Hkey.1, by dsimp only at Herror; omega, Herror.2⟩
  · simp only [allSeen, _root_.decide_false, Bool.not_false, ite_true]
    wp_auto
    ihave Hfields := Hrestore $$ Hall
    have htotal : sint.Z v.pos' + 1 + payload.length + key.length ≤ sint.Z v.buf'.len := by omega
    have hbounds : 0 ≤ sint.Z (W64 (1 + payload.length)) ∧
        sint.Z (W64 (1 + payload.length)) ≤ sint.Z (W64 (1 + payload.length + key.length)) ∧
        sint.Z (W64 (1 + payload.length + key.length)) ≤ sint.Z (v.buf'.len - v.pos') := by word
    have hlo : sint.nat (W64 (1 + payload.length)) = 1 + payload.length := by word
    have hhi : sint.nat (W64 (1 + payload.length + key.length)) = 1 + payload.length + key.length := by word
    have hkey : subslice (sint.nat (W64 (1 + payload.length))) (sint.nat (W64 (1 + payload.length + key.length)))
        (subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs) = key := by
      rw [hbytes, hlo, hhi]
      exact (list_slice3 (head :: payload) key wire (1 + payload.length) (1 + payload.length + key.length)
        (by simp; omega) (by omega) (by omega)).2.1
    simp only [hfields]
    wp_apply wp_fields_match_nested_range fields f names v.buf' xs key v.pos'
      (W64 (1 + payload.length)) (W64 (1 + payload.length + key.length)) qf qn dq hp hbounds hkey
      $$ [Hfields Hnames Hs] as %field %ok ⟨Hfields, Hnames, Hs, %Hmatch⟩
    · iframe Hfields Hnames Hs
    cases ok with
    | false =>
      wp_auto
      wp_apply wp_Skip_failure_bounded c {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} xs wire dq d offset reason kind
        failure hd depth ⟨by rw [Hkey.1]; omega, Hkey.2.1⟩ Hkey.2.2
        $$ [Hc Hs] as %err ⟨Hc, Hs, Herr, %Herror⟩
      · iframe Hkind Hc Hs
      wp_for_post
      iapply HΦ
      iframe Hc Hst Hfields Hnames Hs Herr
      ipureintro
      exact ⟨Hkey.1, by dsimp only at Herror; omega, Herror.2⟩
    | true =>
      have duplicate := ignored field Hmatch
      wp_auto
      wp_apply wp_shiftCount_int
      · ipureintro; exact Hmatch.1
      simp only [duplicate, _root_.decide_false, ite_false]
      wp_auto
      wp_apply wp_Skip_failure_bounded c {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} xs wire dq d offset reason kind
        failure hd depth ⟨by rw [Hkey.1]; omega, Hkey.2.1⟩ Hkey.2.2
        $$ [Hc Hs] as %err ⟨Hc, Hs, Herr, %Herror⟩
      · iframe Hkind Hc Hs
      wp_for_post
      iapply HΦ
      iframe Hc Hst Hfields Hnames Hs Herr
      ipureintro
      exact ⟨Hkey.1, by dsimp only at Herror; omega, Herror.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextWanted_integer_skip_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextWanted_text_skip_rejected
