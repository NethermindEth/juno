import ProofCheck.KeyTextDispatch

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

/-- Normalize the slow path's direct payload slice to the public shortcut's
    nested slice representation; both denote the same borrowed input range. -/
theorem wp_keySlow_text_nested (validator : Utf8ValidatorContract (GF := GF))
    (c : Loc) (v : cursor.Cursor) (xs payload key rem : List w8)
    (head : w8) (dq : DFrac)
    (enc : HeadArgument head payload (W64 key.length))
    (hm : head &&& W8 224 = W8 96)
    (valid : Utf8Valid key)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ key ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"keySlow")) (Val #()))
    {{ RET (PairV (PairV #(slice.slice (slice.slice v.buf' w8 v.pos' v.buf'.len) w8
          (W64 (1 + payload.length)) (W64 (1 + payload.length + key.length))) #true) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + key.length)) = sint.Z v.pos' + 1 + payload.length + key.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + key.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + key.length))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  iintro %Φ ⟨#Hpkg, Hc, Hs⟩ HΦ
  wp_apply wp_keySlow_text validator c v xs payload key rem head dq enc hm valid hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
  · iframe Hc Hs
  have same := slice_slice (V := w8) v.buf' v.pos' v.buf'.len
    (W64 (1 + payload.length)) (W64 (1 + payload.length + key.length)) ⟨by word, by word⟩
  isimp only [same] at HΦ
  iapply HΦ
  iframe Hc Hs
  ipureintro; exact Hnext

/-- Public text Key success at every head width and for arbitrary UTF-8, with
    an explicit unresolved validator contract and no assumed dispatch helpers. -/
theorem wp_Key_text (validator : Utf8ValidatorContract (GF := GF))
    (c : Loc) (v : cursor.Cursor) (xs payload key rem : List w8)
    (head : w8) (dq : DFrac)
    (enc : HeadArgument head payload (W64 key.length))
    (hm : head &&& W8 224 = W8 96)
    (valid : Utf8Valid key)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ key ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ RET (PairV (PairV #(slice.slice (slice.slice v.buf' w8 v.pos' v.buf'.len) w8
          (W64 (1 + payload.length)) (W64 (1 + payload.length + key.length))) #true) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + key.length)) = sint.Z v.pos' + 1 + payload.length + key.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + key.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + key.length))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  by_cases fast : (uint.Z (head &&& W8 31) < 24 ∨ head &&& W8 31 = W8 24) ∧ ASCIIBytes key
  · exact wp_Key_ascii c v xs payload key rem head dq enc hm fast.1 fast.2 hp hbytes
  let Q (returned : GoSlice) (isText : Bool) (err : GoInterface) : IProp GF := iprop(
    ⌜returned = slice.slice (slice.slice v.buf' w8 v.pos' v.buf'.len) w8
        (W64 (1 + payload.length)) (W64 (1 + payload.length + key.length)) ∧
      isText = true ∧ err = interface.nil⌝ ∗
    c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} ∗ v.buf' ↦*{dq} xs ∗
    ⌜sint.Z (v.pos' + W64 (1 + payload.length + key.length)) = sint.Z v.pos' + 1 + payload.length + key.length ∧
      sint.Z (v.pos' + W64 (1 + payload.length + key.length)) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 (1 + payload.length + key.length))) (sint.nat v.buf'.len) xs = rem⌝)
  have slow :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         True ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"keySlow")) (Val #()))
      {{ (returned : GoSlice) (isText : Bool) (err : GoInterface), RET (PairV (PairV #returned #isText) #err);
         Q returned isText err }} := by
    wp_start_folded as ⟨-, Hc, Hs⟩
    wp_apply wp_keySlow_text_nested validator c v xs payload key rem head dq enc hm valid hp hbytes
      $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
    · iframe Hc Hs
    iapply HΦ
    unfold Q
    iframe Hc Hs
    ipureintro; exact ⟨⟨rfl, rfl, rfl⟩, Hnext⟩
  have dispatch :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         True ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
      {{ (returned : GoSlice) (isText : Bool) (err : GoInterface), RET (PairV (PairV #returned #isText) #err);
         Q returned isText err }} := by
    by_cases short : uint.Z (head &&& W8 31) < 24 ∨ head &&& W8 31 = W8 24
    · exact wp_Key_nonascii_slow c v xs payload key rem head dq iprop(True) Q enc hm
        (fun ascii => fast ⟨short, ascii⟩) short hp hbytes slow
    · exact wp_Key_wide_slow c v xs payload key rem head dq iprop(True) Q enc hm
        (by omega) (fun equal => short (Or.inr equal)) hp hbytes slow
  iintro %Φ ⟨#Hpkg, Hc, Hs⟩ HΦ
  wp_apply dispatch $$ [Hc Hs] as %returned %isText %err ⟨%Heq, Hc, Hs, %Hnext⟩
  · iframe Hc Hs
  rcases Heq with ⟨rfl, rfl, rfl⟩
  iapply HΦ
  iframe Hc Hs
  ipureintro; exact Hnext

/-- Public Key rejects every invalid UTF-8 payload atomically, conditional on
    the validator contract. ASCII shortcuts cannot intercept these inputs. -/
theorem wp_Key_invalid_utf8 (validator : Utf8ValidatorContract (GF := GF))
    (c : Loc) (v : cursor.Cursor) (xs payload body rem : List w8)
    (head : w8) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload (W64 body.length))
    (hm : head &&& W8 224 = W8 96) (invalid : ¬ Utf8Valid body)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrSyntax ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"key is not valid UTF-8") slice.nil }} := by
  let R : IProp GF := iprop(globalAddr cursor.ErrSyntax ↦□ kind)
  let Q (returned : GoSlice) (isText : Bool) (err : GoInterface) : IProp GF := iprop(∃ errorLoc : Loc,
    ⌜returned = slice.nil ∧ isText = false ∧ err = interface.mkOk (go.PointerType cursor.DecodeError.ty) #errorLoc⌝ ∗
    c ↦ v ∗ v.buf' ↦*{dq} xs ∗
    errorLoc ↦ cursor.DecodeError.mk kind v.pos' (go!"key is not valid UTF-8") slice.nil)
  have slow :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"keySlow")) (Val #()))
      {{ (returned : GoSlice) (isText : Bool) (err : GoInterface), RET (PairV (PairV #returned #isText) #err);
         Q returned isText err }} := by
    wp_start_folded as ⟨#Hkind, Hc, Hs⟩
    wp_apply wp_keySlow_invalid_utf8 validator c v xs payload body rem head dq kind enc hm invalid hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    iapply HΦ
    unfold Q
    iexists err
    iframe Hc Hs Herr
    ipureintro; exact ⟨rfl, rfl, rfl⟩
  have dispatch :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
      {{ (returned : GoSlice) (isText : Bool) (err : GoInterface), RET (PairV (PairV #returned #isText) #err);
         Q returned isText err }} := by
    by_cases short : uint.Z (head &&& W8 31) < 24 ∨ head &&& W8 31 = W8 24
    · exact wp_Key_nonascii_slow c v xs payload body rem head dq R Q enc hm
        (fun ascii => invalid (utf8Valid_ascii body ascii)) short hp hbytes slow
    · exact wp_Key_wide_slow c v xs payload body rem head dq R Q enc hm
        (by omega) (fun equal => short (Or.inr equal)) hp hbytes slow
  iintro %Φ ⟨#Hpkg, #Hkind, Hc, Hs⟩ HΦ
  wp_apply dispatch $$ [Hc Hs] as %returned %isText %err ⟨%errorLoc, %Heq, Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  rcases Heq with ⟨rfl, rfl, rfl⟩
  iapply HΦ
  iframe Hc Hs Herr

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_keySlow_text_nested
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Key_text
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Key_invalid_utf8
