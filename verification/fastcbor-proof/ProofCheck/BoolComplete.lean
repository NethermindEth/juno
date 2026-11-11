import ProofCheck.BoolRead
import ProofCheck.InitializationErrors

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

/-- The actual typed reader implements null preservation and Boolean assignment,
    with exact, nonwrapping one-byte consumption at any valid offset. -/
theorem wp_readBool_valid (c dst : Loc) (old : Bool) (v : cursor.Cursor)
    (xs item rest : List w8) (value : Bool) (dq : DFrac) (valid : BoolValid old item value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadBool)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ value ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs item rest v.pos' v.buf'.len hp Hlen.1 hbytes
  cases valid with
  | @nullish head hn =>
    wp_auto
    have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    wp_apply wp_null_at c v xs head dq hp.1 hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    wp_end
  | boolean =>
    wp_auto
    have hn : ((if value then W8 245 else W8 244) :: rest).head? ≠ some (W8 246) := by
      cases value <;> simp
    have hu : ((if value then W8 245 else W8 244) :: rest).head? ≠ some (W8 247) := by
      cases value <;> simp
    wp_apply wp_null_not_nullish c v xs ((if value then W8 245 else W8 244) :: rest) dq
      hn hu hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    wp_apply wp_bool_valid c v xs rest value dq hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    wp_end

/-- Failed typed Boolean decoding preserves both the old field and cursor.
    It exposes the actual diagnostic, including tag/type distinction. -/
theorem wp_readBool_rejected (c dst : Loc) (old : Bool) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (reason : BoolFailureReason) (kind : GoInterface)
    (rejected : BoolRejected wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       expectedFailureAddr (GF := GF) (boolExpectedReason reason) ↦□ kind ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadBool)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ BoolFailureError err v.pos' reason kind }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_not_nullish c v xs wire dq rejected.1 rejected.2.1 hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_bool_rejected c v xs wire dq reason kind rejected.2.2 hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

def BoolOutcome (c dst : Loc) (old : Bool) (v : cursor.Cursor) (xs wire : List w8)
    (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (value : Bool), dst ↦ value ∗
    c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ BoolValid old item value ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (reason : BoolFailureReason) (err : Loc), dst ↦ old ∗ c ↦ v ∗
    BoolFailureError err v.pos' reason (expectedFailureKind (boolExpectedReason reason) kinds) ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧ BoolRejected wire reason⌝))

/-- All-input composition for the production typed Boolean reader. No UTF-8
    premise or assumed helper implementation is required. -/
theorem wp_readBool_complete (c dst : Loc) (old : Bool) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (kinds : FeltErrorKinds)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       FeltErrorGlobals kinds ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadBool)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ BoolOutcome c dst old v xs wire kinds result }} := by
  iintro %Φ ⟨#Hpkg, Hglobals, Hdst, Hc, Hs⟩ HΦ
  rcases bool_classify old wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, rejected⟩
  · wp_apply wp_readBool_valid c dst old v xs item rest value dq valid hp (hbytes.trans split)
      $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs, %Hnext⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    unfold BoolOutcome
    ileft
    iexists item, rest, value
    iframe Hdst Hc
    ipureintro
    exact ⟨rfl, split, valid, Hnext⟩
  · ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 0) (boolExpectedReason reason)) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind] at Hkind
    wp_apply wp_readBool_rejected c dst old v xs wire dq reason
      (expectedFailureKind (boolExpectedReason reason) kinds) rejected hp hbytes
      $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    iapply HΦ
    iframe Hs
    unfold BoolOutcome
    iright
    iexists reason, err
    iframe Hdst Hc Herr
    ipureintro
    exact ⟨rfl, rejected⟩

/-- The actual package initialization supplies the diagnostic category objects. -/
theorem wp_readBool_initialized (c dst : Loc) (old : Bool) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadBool)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds,
       FeltErrorGlobals kinds ∗ BoolOutcome c dst old v xs wire kinds result }} := by
  iintro %Φ ⟨#Hpkg, Hdst, Hc, Hs⟩ HΦ
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ Hpkg
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  wp_apply wp_readBool_complete c dst old v xs wire dq kinds hp hbytes
    $$ [Hdst Hc Hs] as %result ⟨Hs, Hout⟩
  · iframe Hglobals Hdst Hc Hs
  iapply HΦ
  iframe Hs
  iexists kinds
  iframe Hglobals Hout

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readBool_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readBool_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readBool_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readBool_initialized
