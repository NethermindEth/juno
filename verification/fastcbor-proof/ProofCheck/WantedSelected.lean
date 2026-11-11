import ProofCheck.WantedBoundary
import ProofCheck.KeyFields

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

/-- The actual projection scanner consumes any supported spelling/width of a
    fresh selected key, marks exactly that field and leaves its value unread.
    Metadata and the complete input are preserved with fractional ownership. -/
theorem wp_nextWanted_selected (validator : Utf8ValidatorContract (GF := GF))
    (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (names : List GoString) (name : GoString) (xs payload key rem : List w8) (head : w8)
    (index : w64) (qf qn dq : DFrac)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left') (incomplete : s.seen' ≠ f.all')
    (distinct : (names.map (List.map fieldFoldByte)).Nodup)
    (hi : 0 ≤ sint.Z index ∧ sint.Z index < 64)
    (lookup : names[(sint.Z index).toNat]? = some name)
    (matched : key.map fieldFoldByte = name.map fieldFoldByte)
    (fresh : s.seen' &&& (W64 1 <<< index) = W64 0)
    (enc : HeadArgument head payload (W64 key.length)) (major : head &&& W8 224 = W8 96)
    (valid : Utf8Valid key)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ key ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextWanted")) (Val #st))
    {{ RET (PairV #index #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} ∗
       st ↦ {s with left' := s.left' - W64 1, seen' := s.seen' ||| (W64 1 <<< index)} ∗
       fields ↦{qf} f ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + key.length)) = sint.Z v.pos' + 1 + payload.length + key.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + key.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + key.length))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hc, Hst, Hfields, Hnames, Hs⟩
  wp_auto
  wp_for
  have hz : sint.Z (W64 0) = 0 := rfl
  simp only [hz, hleft, _root_.decide_true, ite_true]
  wp_auto
  wp_apply wp_Key_text validator c v xs payload key rem head dq enc major valid hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
  · iframe Hc Hs
  simp only [hfields]
  icases (cursor.Fields.Fields_access_load_all fields f qf).access_strict $$ Hfields with ⟨Hall, Hrestore⟩
  wp_auto
  simp only [incomplete, _root_.decide_false, Bool.false_eq_true, ite_false]
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
    exact (list_slice3 (head :: payload) key rem (1 + payload.length) (1 + payload.length + key.length)
      (by simp; omega) (by omega) (by omega)).2.1
  simp only [hfields]
  wp_apply wp_fields_match_nested_range fields f names v.buf' xs key v.pos'
    (W64 (1 + payload.length)) (W64 (1 + payload.length + key.length)) qf qn dq hp hbounds hkey
    $$ [Hfields Hnames Hs] as %field %ok ⟨Hfields, Hnames, Hs, %Hmatch⟩
  · iframe Hfields Hnames Hs
  cases ok with
  | false => exact False.elim (Hmatch.2 name (List.mem_of_getElem? lookup) matched)
  | true =>
    have same : field = index := fieldMatchResult_unique names key field index distinct Hmatch ⟨hi.1, name, lookup, matched⟩
    subst field
    wp_auto
    wp_apply wp_shiftCount_int
    · ipureintro; exact hi.1
    simp only [fresh]
    wp_auto
    wp_apply wp_shiftCount_int
    · ipureintro; exact hi.1
    wp_for_post
    iapply HΦ
    isimp only [hfields] at Hst
    iframe Hc Hst Hfields Hnames Hs
    ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextWanted_selected
