import ProofCheck.Errors
import Perennial.Golang.Theory.String

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 1600000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

def headDescription (b : w8) : GoString :=
  if b &&& W8 224 = W8 0 then go!"an unsigned integer"
  else if b &&& W8 224 = W8 32 then go!"a negative integer"
  else if b &&& W8 224 = W8 64 then go!"a byte string"
  else if b &&& W8 224 = W8 96 then go!"a text string"
  else if b &&& W8 224 = W8 128 then go!"an array"
  else if b &&& W8 224 = W8 160 then go!"a map"
  else if b &&& W8 224 = W8 192 then go!"a tag"
  else (go!"a simple value or float (head 0x") ++
    [(go!"0123456789abcdef")[(uint.Z (b >>> W64 4)).toNat]!,
     (go!"0123456789abcdef")[(uint.Z (b &&& W8 15)).toNat]!] ++ (go!")")

/-- Diagnostic construction executes for every byte, including simple and float heads. -/
theorem wp_describe (b : w8) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! cursor.describe)) (Val #b))
    {{ RET #(headDescription b); True }} := by
  wp_start
  wp_auto
  by_cases h0 : b &&& W8 224 = W8 0
  · simp only [h0, _root_.decide_true]
    wp_auto
    have hd : headDescription b = go!"an unsigned integer" := by
      simp only [headDescription, h0] <;> rfl
    isimp only [hd] at HΦ
    wp_end
  simp only [h0, _root_.decide_false]
  wp_auto
  by_cases h1 : b &&& W8 224 = W8 32
  · simp only [h1, _root_.decide_true]
    wp_auto
    have hd : headDescription b = go!"a negative integer" := by
      simp only [headDescription, h1] <;> rfl
    isimp only [hd] at HΦ
    wp_end
  simp only [h1, _root_.decide_false]
  wp_auto
  by_cases h2 : b &&& W8 224 = W8 64
  · simp only [h2, _root_.decide_true]
    wp_auto
    have hd : headDescription b = go!"a byte string" := by
      simp only [headDescription, h2] <;> rfl
    isimp only [hd] at HΦ
    wp_end
  simp only [h2, _root_.decide_false]
  wp_auto
  by_cases h3 : b &&& W8 224 = W8 96
  · simp only [h3, _root_.decide_true]
    wp_auto
    have hd : headDescription b = go!"a text string" := by
      simp only [headDescription, h3] <;> rfl
    isimp only [hd] at HΦ
    wp_end
  simp only [h3, _root_.decide_false]
  wp_auto
  by_cases h4 : b &&& W8 224 = W8 128
  · simp only [h4, _root_.decide_true]
    wp_auto
    have hd : headDescription b = go!"an array" := by
      simp only [headDescription, h4] <;> rfl
    isimp only [hd] at HΦ
    wp_end
  simp only [h4, _root_.decide_false]
  wp_auto
  by_cases h5 : b &&& W8 224 = W8 160
  · simp only [h5, _root_.decide_true]
    wp_auto
    have hd : headDescription b = go!"a map" := by
      simp only [headDescription, h5] <;> rfl
    isimp only [hd] at HΦ
    wp_end
  simp only [h5, _root_.decide_false]
  wp_auto
  by_cases h6 : b &&& W8 224 = W8 192
  · simp only [h6, _root_.decide_true]
    wp_auto
    have hd : headDescription b = go!"a tag" := by
      simp only [headDescription, h6] <;> rfl
    isimp only [hd] at HΦ
    wp_end
  simp only [h6, _root_.decide_false]
  wp_auto
  have idxHi : (if 0 ≤ sint.Z (W64 (uint.Z (b >>> W64 4))) then
      (go!"0123456789abcdef")[sint.nat (W64 (uint.Z (b >>> W64 4)))]? else none) =
      some ((go!"0123456789abcdef")[(uint.Z (b >>> W64 4)).toNat]!) := by
    have h : ∀ b : w8, (if 0 ≤ sint.Z (W64 (uint.Z (b >>> W64 4))) then
      (go!"0123456789abcdef")[sint.nat (W64 (uint.Z (b >>> W64 4)))]? else none) =
      some ((go!"0123456789abcdef")[(uint.Z (b >>> W64 4)).toNat]!) := by decide
    exact h b
  simp only [idxHi]
  wp_auto
  have idxLo : (if 0 ≤ sint.Z (W64 (uint.Z (b &&& W8 15))) then
      (go!"0123456789abcdef")[sint.nat (W64 (uint.Z (b &&& W8 15)))]? else none) =
      some ((go!"0123456789abcdef")[(uint.Z (b &&& W8 15)).toNat]!) := by
    have h : ∀ b : w8, (if 0 ≤ sint.Z (W64 (uint.Z (b &&& W8 15))) then
      (go!"0123456789abcdef")[sint.nat (W64 (uint.Z (b &&& W8 15)))]? else none) =
      some ((go!"0123456789abcdef")[(uint.Z (b &&& W8 15)).toNat]!) := by decide
    exact h b
  simp only [idxLo]
  wp_auto
  wp_apply wp_slice_literal (V := w8)
    [(go!"0123456789abcdef")[(uint.Z (b >>> W64 4)).toNat]!,
     (go!"0123456789abcdef")[(uint.Z (b &&& W8 15)).toNat]!]
  isplitr
  · ipureintro; rfl
  iintro %sl ⟨Hsl, -⟩
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (2 : Int) 0) 0 = 2 from by decide]
  wp_apply wp_bytes_to_string (slice.mk sl (W64 2) (W64 2)) _ (DFrac.own 1) $$ [Hsl] with Hsl
  · iframe Hsl
  simp only [headDescription, h0, h1, h2, h3, h4, h5, h6, ite_false]
  wp_end

/-- A tag encountered in a typed position is unsupported, not a type mismatch. -/
theorem wp_mismatch_tag (c : Loc) (v : cursor.Cursor) (want : GoString) (head : w8)
    (kind : GoInterface) (htag : head &&& W8 224 = W8 192) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v }}
      (App (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"mismatch")) (Val #want)) (Val #head))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"tag where %s is expected") args ∗
       args ↦* [interface.mkOk go.string #want] }} := by
  wp_start as ⟨#Hkind, Hc⟩
  wp_auto
  simp only [htag, _root_.decide_true]
  wp_auto
  wp_apply wp_slice_literal (V := GoInterface) [interface.mkOk go.string #want]
  isplitr
  · ipureintro; rfl
  iintro %args ⟨Hargs, -⟩
  wp_auto
  wp_apply wp_fail c v kind (go!"tag where %s is expected") _ $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc
  wp_end

/-- All other mismatched major types preserve the cursor and own both diagnostic strings. -/
theorem wp_mismatch_type (c : Loc) (v : cursor.Cursor) (want : GoString) (head : w8)
    (kind : GoInterface) (htag : head &&& W8 224 ≠ W8 192) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrType ↦□ kind ∗ c ↦ v }}
      (App (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"mismatch")) (Val #want)) (Val #head))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"expected %s, found %s") args ∗
       args ↦* [interface.mkOk go.string #want, interface.mkOk go.string #(headDescription head)] }} := by
  wp_start as ⟨#Hkind, Hc⟩
  wp_auto
  simp only [htag, _root_.decide_false]
  wp_auto
  wp_apply wp_describe head
  wp_apply wp_slice_literal (V := GoInterface)
    [interface.mkOk go.string #want, interface.mkOk go.string #(headDescription head)]
  isplitr
  · ipureintro; rfl
  iintro %args ⟨Hargs, -⟩
  wp_auto
  wp_apply wp_fail c v kind (go!"expected %s, found %s") _ $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc
  wp_end

/-- Invalid head rejection preserves the cursor and reports its original offset. -/
theorem wp_headSlow_indefinite (c : Loc) (v : cursor.Cursor) (xs : List w8)
    (head : w8) (dq : DFrac) (kind : GoInterface)
    (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hinfo : 27 < uint.Z (head &&& W8 31)) (hindef : head &&& W8 31 = W8 31)
    (hmajor : 64 ≤ uint.Z (head &&& W8 224) ∧ uint.Z (head &&& W8 224) ≤ 160) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"headSlow")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #(W8 0) #(W64 0))
        #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"indefinite-length item") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  have hnend : ¬ sint.Z v.buf'.len ≤ sint.Z v.pos' := by omega
  have hnsmall : ¬ uint.Z (head &&& W8 31) < uint.Z (W8 24) := by change ¬ uint.Z (head &&& W8 31) < 24; omega
  have hlarge : uint.Z (W8 27) < uint.Z (head &&& W8 31) := hinfo
  wp_auto
  simp only [hnend, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  simp only [hnsmall, _root_.decide_false]
  wp_auto
  simp only [hlarge, _root_.decide_true]
  wp_auto
  have hlo : uint.Z (W8 64) ≤ uint.Z (head &&& W8 224) := hmajor.1
  have hhi : uint.Z (head &&& W8 224) ≤ uint.Z (W8 160) := hmajor.2
  repeat (first | simp only [hindef, hlo, hhi, _root_.decide_true] | wp_auto)
  wp_apply wp_fail c v kind (go!"indefinite-length item") slice.nil $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc
  wp_end

/-- Invalid head rejection preserves the cursor and reports its original offset. -/
theorem wp_headSlow_break (c : Loc) (v : cursor.Cursor) (xs : List w8)
    (head : w8) (dq : DFrac) (kind : GoInterface)
    (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hinfo : 27 < uint.Z (head &&& W8 31)) (hindef : head &&& W8 31 = W8 31)
    (hmajor : head &&& W8 224 = W8 224) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrSyntax ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"headSlow")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #(W8 0) #(W64 0))
        #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"break code outside an indefinite-length item") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  have hnend : ¬ sint.Z v.buf'.len ≤ sint.Z v.pos' := by omega
  have hnsmall : ¬ uint.Z (head &&& W8 31) < uint.Z (W8 24) := by change ¬ uint.Z (head &&& W8 31) < 24; omega
  have hlarge : uint.Z (W8 27) < uint.Z (head &&& W8 31) := hinfo
  wp_auto
  simp only [hnend, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  simp only [hnsmall, _root_.decide_false]
  wp_auto
  simp only [hlarge, _root_.decide_true]
  wp_auto
  simp only [hindef, hmajor]
  wp_auto
  wp_apply wp_fail c v kind (go!"break code outside an indefinite-length item") slice.nil $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc
  wp_end

/-- Reserved additional information and illegal indefinite heads are syntax errors. -/
theorem wp_headSlow_invalid (c : Loc) (v : cursor.Cursor) (xs : List w8)
    (head : w8) (dq : DFrac) (kind : GoInterface)
    (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hinfo : 27 < uint.Z (head &&& W8 31))
    (hindef : ¬ (head &&& W8 31 = W8 31 ∧
      64 ≤ uint.Z (head &&& W8 224) ∧ uint.Z (head &&& W8 224) ≤ 160))
    (hbreak : ¬ (head &&& W8 31 = W8 31 ∧ head &&& W8 224 = W8 224)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrSyntax ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"headSlow")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #(W8 0) #(W64 0))
        #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"invalid additional information %d for %s") args ∗
       args ↦* [interface.mkOk go.byte #(head &&& W8 31),
         interface.mkOk go.string #(headDescription head)] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  have hnend : ¬ sint.Z v.buf'.len ≤ sint.Z v.pos' := by omega
  have hnsmall : ¬ uint.Z (head &&& W8 31) < uint.Z (W8 24) := by change ¬ uint.Z (head &&& W8 31) < 24; omega
  have hlarge : uint.Z (W8 27) < uint.Z (head &&& W8 31) := hinfo
  wp_auto
  simp only [hnend, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  simp only [hnsmall, _root_.decide_false]
  wp_auto
  simp only [hlarge, _root_.decide_true]
  wp_auto
  by_cases heq : head &&& W8 31 = W8 31
  · have hnmajor : head &&& W8 224 ≠ W8 224 := by intro h; exact hbreak ⟨heq, h⟩
    by_cases hlo : uint.Z (W8 64) ≤ uint.Z (head &&& W8 224)
    · have hnhi : ¬ uint.Z (head &&& W8 224) ≤ uint.Z (W8 160) := by
        intro h; exact hindef ⟨heq, hlo, h⟩
      repeat (first | simp only [heq, hlo, hnhi, hnmajor, _root_.decide_true, _root_.decide_false] | wp_auto)
      wp_apply wp_describe head
      wp_apply wp_slice_literal (V := GoInterface)
        [interface.mkOk go.byte #(W8 31), interface.mkOk go.string #(headDescription head)]
      isplitr
      · ipureintro; rfl
      iintro %args ⟨Hargs, -⟩
      wp_auto
      wp_apply wp_fail c v kind (go!"invalid additional information %d for %s") _ $$ [Hc] as %err ⟨Hc, Herr⟩
      · iframe Hc
      wp_end
    · repeat (first | simp only [heq, hlo, hnmajor, _root_.decide_true, _root_.decide_false] | wp_auto)
      wp_apply wp_describe head
      wp_apply wp_slice_literal (V := GoInterface)
        [interface.mkOk go.byte #(W8 31), interface.mkOk go.string #(headDescription head)]
      isplitr
      · ipureintro; rfl
      iintro %args ⟨Hargs, -⟩
      wp_auto
      wp_apply wp_fail c v kind (go!"invalid additional information %d for %s") _ $$ [Hc] as %err ⟨Hc, Herr⟩
      · iframe Hc
      wp_end
  · repeat (first | simp only [heq, _root_.decide_false] | wp_auto)
    wp_apply wp_describe head
    wp_apply wp_slice_literal (V := GoInterface)
      [interface.mkOk go.byte #(head &&& W8 31), interface.mkOk go.string #(headDescription head)]
    isplitr
    · ipureintro; rfl
    iintro %args ⟨Hargs, -⟩
    wp_auto
    wp_apply wp_fail c v kind (go!"invalid additional information %d for %s") _ $$ [Hc] as %err ⟨Hc, Herr⟩
    · iframe Hc
    wp_end

/-- A successfully parsed head of the wrong type is rolled back before reporting the mismatch. -/
theorem wp_expectSlow_tag (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head want : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 ≠ want) (htag : head &&& W8 224 = W8 192)
    (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"expectSlow")) (Val #want))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"tag where %s is expected") args ∗
       args ↦* [interface.mkOk go.string #(headDescription want)] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by omega
  wp_auto
  wp_apply wp_headSlow_encoded c v xs payload rem head argument dq enc hp hbound hi hbytes
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [hm, _root_.decide_false]
  wp_auto
  wp_apply wp_describe want
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  wp_apply wp_mismatch_tag c v (headDescription want) head kind htag $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc Hkind
  wp_end

/-- A successfully parsed head of the wrong type is rolled back before reporting the mismatch. -/
theorem wp_expectSlow_type (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head want : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 ≠ want) (htag : head &&& W8 224 ≠ W8 192)
    (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrType ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"expectSlow")) (Val #want))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"expected %s, found %s") args ∗
       args ↦* [interface.mkOk go.string #(headDescription want), interface.mkOk go.string #(headDescription head)] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by omega
  wp_auto
  wp_apply wp_headSlow_encoded c v xs payload rem head argument dq enc hp hbound hi hbytes
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [hm, _root_.decide_false]
  wp_auto
  wp_apply wp_describe want
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  wp_apply wp_mismatch_type c v (headDescription want) head kind htag $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc Hkind
  wp_end

/-- No byte of another major type can enter expect's inline fast path. -/
theorem byte_wrong_major_slow (head want : w8) (hw : want &&& W8 224 = want)
    (hm : head &&& W8 224 ≠ want) : ¬ uint.Z (head - want) < uint.Z (W8 24) := by
  have h : ∀ want : w8, want &&& W8 224 = want → ∀ head : w8,
      head &&& W8 224 ≠ want → ¬ uint.Z (head - want) < uint.Z (W8 24) := by decide
  exact h want hw head hm

/-- The fast expectation guard also rejects every mismatched major type. -/
theorem wp_expect_tag (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head want : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hw : want &&& W8 224 = want) (hm : head &&& W8 224 ≠ want) (htag : head &&& W8 224 = W8 192)
    (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"expect")) (Val #want))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"tag where %s is expected") args ∗
       args ↦* [interface.mkOk go.string #(headDescription want)] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by omega
  have hsmall := byte_wrong_major_slow head want hw hm
  wp_auto
  simp only [hlt, _root_.decide_true]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  simp only [hsmall, _root_.decide_false]
  wp_auto
  wp_apply wp_expectSlow_tag c v xs payload rem head want argument dq kind enc hm htag hp hbound hi hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Public unsigned decoding preserves the cursor and input on a type rejection. -/
theorem wp_uint64_tag (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 ≠ W8 0) (htag : head &&& W8 224 = W8 192)
    (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Uint64")) (Val #()))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"tag where %s is expected") args ∗
       args ↦* [interface.mkOk go.string #(headDescription (W8 0))] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_expect_tag c v xs payload rem head (W8 0) argument dq kind enc (by decide) hm htag hp hbound hi hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- The fast expectation guard also rejects every mismatched major type. -/
theorem wp_expect_type (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head want : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hw : want &&& W8 224 = want) (hm : head &&& W8 224 ≠ want) (htag : head &&& W8 224 ≠ W8 192)
    (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrType ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"expect")) (Val #want))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"expected %s, found %s") args ∗
       args ↦* [interface.mkOk go.string #(headDescription want), interface.mkOk go.string #(headDescription head)] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by omega
  have hsmall := byte_wrong_major_slow head want hw hm
  wp_auto
  simp only [hlt, _root_.decide_true]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  simp only [hsmall, _root_.decide_false]
  wp_auto
  wp_apply wp_expectSlow_type c v xs payload rem head want argument dq kind enc hm htag hp hbound hi hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Public unsigned decoding preserves the cursor and input on a type rejection. -/
theorem wp_uint64_type (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 ≠ W8 0) (htag : head &&& W8 224 ≠ W8 192)
    (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrType ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Uint64")) (Val #()))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"expected %s, found %s") args ∗
       args ↦* [interface.mkOk go.string #(headDescription (W8 0)), interface.mkOk go.string #(headDescription head)] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_expect_type c v xs payload rem head (W8 0) argument dq kind enc (by decide) hm htag hp hbound hi hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Typed scalar fields remain untouched on a non-null type rejection. -/
theorem wp_readUint64_tag (c dst : Loc) (old : w64) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 ≠ W8 0) (htag : head &&& W8 224 = W8 192)
    (hn : head ≠ W8 246) (hu : head ≠ W8 247)
    (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadUint64)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"tag where %s is expected") args ∗
       args ↦* [interface.mkOk go.string #(headDescription (W8 0))] }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_other c v xs head dq hp hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_uint64_tag c v xs payload rem head argument dq kind enc hm htag hp hbound hi hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Typed scalar fields remain untouched on a non-null type rejection. -/
theorem wp_readMode_tag (c dst : Loc) (old : L1DAMode) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 ≠ W8 0) (htag : head &&& W8 224 = W8 192)
    (hn : head ≠ W8 246) (hu : head ≠ W8 247)
    (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadMode)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"tag where %s is expected") args ∗
       args ↦* [interface.mkOk go.string #(headDescription (W8 0))] }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_other c v xs head dq hp hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_uint64_tag c v xs payload rem head argument dq kind enc hm htag hp hbound hi hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Typed scalar fields remain untouched on a non-null type rejection. -/
theorem wp_readUint64_type (c dst : Loc) (old : w64) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 ≠ W8 0) (htag : head &&& W8 224 ≠ W8 192)
    (hn : head ≠ W8 246) (hu : head ≠ W8 247)
    (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cursor.ErrType ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadUint64)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"expected %s, found %s") args ∗
       args ↦* [interface.mkOk go.string #(headDescription (W8 0)), interface.mkOk go.string #(headDescription head)] }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_other c v xs head dq hp hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_uint64_type c v xs payload rem head argument dq kind enc hm htag hp hbound hi hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Typed scalar fields remain untouched on a non-null type rejection. -/
theorem wp_readMode_type (c dst : Loc) (old : L1DAMode) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 ≠ W8 0) (htag : head &&& W8 224 ≠ W8 192)
    (hn : head ≠ W8 246) (hu : head ≠ W8 247)
    (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cursor.ErrType ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadMode)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"expected %s, found %s") args ∗
       args ↦* [interface.mkOk go.string #(headDescription (W8 0)), interface.mkOk go.string #(headDescription head)] }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_other c v xs head dq hp hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_uint64_type c v xs payload rem head argument dq kind enc hm htag hp hbound hi hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- A truncated head is reported before any expected-type comparison. -/
theorem wp_expectSlow_truncated (c : Loc) (v : cursor.Cursor) (xs : List w8)
    (head want : w8) (size : w64) (dq : DFrac) (kind : GoInterface)
    (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hinfo : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
    (hsize : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
    (htrunc : sint.Z v.buf'.len - (sint.Z v.pos' + 1) < sint.Z size) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"expectSlow")) (Val #want))
    {{ (err : Loc), RET (PairV #(W64 0)
        #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends inside the head") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_headSlow_truncated c v xs head size dq kind hp hlt hi hinfo hsize htrunc
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Inline fast-path selection cannot hide a truncated extended argument. -/
theorem wp_expect_truncated (c : Loc) (v : cursor.Cursor) (xs : List w8)
    (head want : w8) (size : w64) (dq : DFrac) (kind : GoInterface)
    (hw : want &&& W8 224 = want) (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hinfo : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
    (hsize : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
    (htrunc : sint.Z v.buf'.len - (sint.Z v.pos' + 1) < sint.Z size) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"expect")) (Val #want))
    {{ (err : Loc), RET (PairV #(W64 0)
        #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends inside the head") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  have hsmall : ¬ uint.Z (head - want) < uint.Z (W8 24) := by
    by_cases hm : head &&& W8 224 = want
    · rw [← hm, byte_major_remainder]
      change ¬ uint.Z (head &&& W8 31) < 24
      omega
    · exact byte_wrong_major_slow head want hw hm
  wp_auto
  simp only [hlt, _root_.decide_true]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  simp only [hsmall, _root_.decide_false]
  wp_auto
  wp_apply wp_expectSlow_truncated c v xs head want size dq kind hp hlt hi hinfo hsize htrunc
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Public unsigned decoding preserves the cursor and input on a truncated argument. -/
theorem wp_uint64_truncated (c : Loc) (v : cursor.Cursor) (xs : List w8)
    (head : w8) (size : w64) (dq : DFrac) (kind : GoInterface)
    (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hinfo : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
    (hsize : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
    (htrunc : sint.Z v.buf'.len - (sint.Z v.pos' + 1) < sint.Z size) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Uint64")) (Val #()))
    {{ (err : Loc), RET (PairV #(W64 0)
        #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends inside the head") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_expect_truncated c v xs head (W8 0) size dq kind (by decide) hp hlt hi hinfo hsize htrunc
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Typed scalar fields remain untouched on a truncated argument. -/
theorem wp_readUint64_truncated (c dst : Loc) (old : w64) (v : cursor.Cursor) (xs : List w8)
    (head : w8) (size : w64) (dq : DFrac) (kind : GoInterface)
    (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hinfo : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
    (hsize : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
    (htrunc : sint.Z v.buf'.len - (sint.Z v.pos' + 1) < sint.Z size) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadUint64)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends inside the head") slice.nil }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  have hn : head ≠ W8 246 := by intro h; subst head; contradiction
  have hu : head ≠ W8 247 := by intro h; subst head; contradiction
  wp_auto
  wp_apply wp_null_other c v xs head dq hp hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_uint64_truncated c v xs head size dq kind hp hlt hi hinfo hsize htrunc
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Typed scalar fields remain untouched on a truncated argument. -/
theorem wp_readMode_truncated (c dst : Loc) (old : L1DAMode) (v : cursor.Cursor) (xs : List w8)
    (head : w8) (size : w64) (dq : DFrac) (kind : GoInterface)
    (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hinfo : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
    (hsize : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
    (htrunc : sint.Z v.buf'.len - (sint.Z v.pos' + 1) < sint.Z size) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadMode)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends inside the head") slice.nil }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  have hn : head ≠ W8 246 := by intro h; subst head; contradiction
  have hu : head ≠ W8 247 := by intro h; subst head; contradiction
  wp_auto
  wp_apply wp_null_other c v xs head dq hp hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_uint64_truncated c v xs head size dq kind hp hlt hi hinfo hsize htrunc
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_describe

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_mismatch_tag
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_mismatch_type

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headSlow_indefinite

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headSlow_break

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headSlow_invalid

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_expectSlow_tag

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_expectSlow_type

#print axioms Perennial.github_com.NethermindEth.juno.core.byte_wrong_major_slow

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_expect_tag

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_expect_type

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_uint64_tag

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_uint64_type

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readUint64_tag

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readMode_tag

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readUint64_type

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readMode_type

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_expectSlow_truncated

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_expect_truncated

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_uint64_truncated

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readUint64_truncated

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readMode_truncated
