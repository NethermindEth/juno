import ProofCheck.FeltReject

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- After four limbs and any valid ignored prefix, an actual Skip failure
    restores the array start and depth without writing the destination. -/
theorem wp_limbsSlow_extras_reject (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (count : w64)
    (items : List UnsignedWire) (extras : List (List w8))
    (dq : DFrac) (R : IProp GF) (E : Loc → IProp GF)
    (d : Nat) (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hcount : items.length = 4) (hmore : 4 + extras.length < uint.Z count)
    (valid : ∀ item ∈ extras, SkipValid (d + 1) item)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ extras.flatten ++ rem)
    (hbad :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length), depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
      {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
         c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length), depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs ∗ E err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"limbsSlow")) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  wp_start as ⟨HR, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hw := headArgument_width head payload count enc
  have hu := unsignedBytes_lower_bound items
  have he := skipValid_items_bound (d + 1) extras valid
  have htotal : sint.Z v.pos' + 1 + payload.length + (unsignedBytes items).length + extras.flatten.length ≤ sint.Z v.buf'.len := by
    have hlength := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
      List.length_append, List.length_cons] at hlength
    simp only [sint.nat, sint.Z] at Hlen hlength hp ⊢
    omega
  have hcountfit : (4 : Int) + extras.length ≤ sint.Z v.buf'.len := by omega
  have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by omega
  have hi := input_suffix_head xs (payload ++ unsignedBytes items ++ extras.flatten ++ rem)
    (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs =
      payload ++ (unsignedBytes items ++ extras.flatten ++ rem) := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _
      (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  rw [← hm]
  wp_apply wp_expect_encoded c v xs payload (unsignedBytes items ++ extras.flatten ++ rem) head count dq
    enc hp.1 hbound hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_enter c {v with pos' := v.pos' + W64 (1 + payload.length)} v.pos' (by change sint.Z v.depth' < 32; omega)
    $$ [Hc] with Hc
  · iframe Hc
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs =
      unsignedBytes items ++ extras.flatten ++ rem := by
    have hn : sint.nat (v.pos' + W64 (1 + payload.length)) = sint.nat v.pos' + (head :: payload).length := by
      simp only [List.length_cons]; word
    rw [hn]
    exact input_suffix_after xs (head :: payload) _ _ _ (by simpa only [List.append_assoc] using hbytes)
  ihave HI : (∃ (done todo : List UnsignedWire) (passed pending : List (List w8)) (pos oldi : w64),
      "Hrange" ∷ «$range_i_ptr» ↦ W64 (done.length + passed.length) ∗
      "Hi" ∷ i_ptr ↦ oldi ∗
      "Hlimbs" ∷ limbs_ptr ↦ paddedLimbs done ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hsplit" ∷ ⌜items = done ++ todo⌝ ∗
      "%Hextras" ∷ ⌜extras = passed ++ pending⌝ ∗
      "%Horder" ∷ ⌜done.length < 4 → passed = []⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + 1 + payload.length + (unsignedBytes done).length + passed.flatten.length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = unsignedBytes todo ++ pending.flatten ++ rem⌝ : IProp GF)
      $$ [«$range_i» i limbs Hc]
  · iexists [], items, [], extras, (v.pos' + W64 (1 + payload.length)), (W64 0)
    iframe
    ipureintro
    exact ⟨rfl, rfl, by simp, ⟨by word, by word⟩, by simpa [unsignedBytes] using hstart, hremaining⟩
  wp_for HI
  have hlength : done.length + todo.length = 4 := by rw [← hcount, Hsplit, List.length_append]
  have hextralength : extras.length = passed.length + pending.length := by rw [Hextras, List.length_append]
  have hdone : done.length ≤ 4 := by omega
  have hcond : uint.Z (W64 (done.length + passed.length)) < uint.Z count := by word
  simp only [hcond, _root_.decide_true, ite_true]
  wp_auto
  by_cases hfull : 4 ≤ done.length + passed.length
  · have htest : uint.Z (W64 4) ≤ uint.Z (W64 (done.length + passed.length)) := by word
    simp only [htest, _root_.decide_true]
    wp_auto
    have hfour : done.length = 4 := by
      by_cases hn : done.length = 4
      · exact hn
      have heq := Horder (by omega)
      simp only [heq, List.length_nil] at hfull
      omega
    have htodo : todo = [] := List.eq_nil_of_length_eq_zero (by omega)
    cases pending with
    | nil =>
      have hdoneall : done = items := by simpa [htodo] using Hsplit.symm
      have hpassedall : passed = extras := by simpa using Hextras.symm
      have hposfinal : pos = v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length) := by
        rw [hdoneall, hpassedall] at Hposition
        word
      isimp only [hposfinal] at Hc
      wp_apply hbad $$ [HR Hc Hs] as %err ⟨Hc, Hs, Herr⟩
      · iframe HR Hc Hs
      wp_for_post
      have hrestoredepth : v.depth' + W64 1 - W64 1 = v.depth' := by word
      isimp only [hrestoredepth] at Hc
      iapply HΦ
      iframe Hdst Hc Hs Herr
    | cons item pending =>
      have hmember : item ∈ extras := by rw [Hextras]; simp
      have hitem : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = item ++ (pending.flatten ++ rem) := by
        simpa only [htodo, unsignedBytes, List.flatMap_nil, List.nil_append, List.flatten_cons, List.append_assoc] using Hremaining
      wp_apply wp_Skip_valid c {v with pos' := pos, depth' := v.depth' + W64 1} xs item (pending.flatten ++ rem) dq
        (d + 1) (by change sint.Z (v.depth' + W64 1) = (d + 1 : Nat); word) (by omega) (valid item hmember) Hpos hitem
        $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
      · iframe Hc Hs
      ieval (change (c ↦ {v with pos' := pos + W64 item.length, depth' := v.depth' + W64 1} : IProp GF) ⊢ _) at Hc
      dsimp only at Hnext
      have hbyteslen : (passed ++ [item]).flatten.length = passed.flatten.length + item.length := by simp
      have hinc : W64 (done.length + passed.length) + W64 1 = W64 (done.length + (passed ++ [item]).length) := by
        simp only [List.length_append, List.length_singleton]; word
      wp_for_post
      isimp only [hinc] at Hrange
      iframe
      iexists done, todo, (passed ++ [item]), pending, (pos + W64 item.length), (W64 (done.length + passed.length))
      iframe
      ipureintro
      refine ⟨Hsplit, ?_, ?_, ⟨?_, Hnext.2.1⟩, ?_, ?_⟩
      · simpa only [List.append_assoc, List.singleton_append] using Hextras
      · intro hn; omega
      · rw [Hnext.1]; omega
      · rw [Hnext.1, hbyteslen]; omega
      · simpa only [htodo, unsignedBytes, List.flatMap_nil, List.nil_append] using Hnext.2.2
  · have hnotfull : ¬ uint.Z (W64 4) ≤ uint.Z (W64 (done.length + passed.length)) := by word
    simp only [hnotfull, _root_.decide_false]
    wp_auto
    have hsmall : done.length < 4 := by omega
    have hpassed := Horder hsmall
    cases todo with
    | nil => simp only [List.length_nil] at hlength; omega
    | cons item todo =>
      have hindex : sint.Z (W64 (done.length + passed.length)) = done.length := by rw [hpassed]; simp only [List.length_nil]; word
      rw [ite_eq_left_of_eq_true _ _ (eq_true (by rw [hindex]; exact ⟨by omega, by omega⟩))]
      wp_auto
      wp_apply wp_uint64_wire c {v with pos' := pos, depth' := v.depth' + W64 1} xs
          (unsignedBytes todo ++ pending.flatten ++ rem) item dq Hpos
          (by simpa [unsignedBytes, List.append_assoc] using Hremaining)
          $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
      · iframe Hc Hs
      icases array_acc limbs_ptr (done.length : Int) (DFrac.own 1) 4 (paddedLimbs done) (W64 0)
          (by omega) (by simpa using paddedLimbs_next done hsmall) $$ Hlimbs with ⟨Hword, Hrestore⟩
      simp only [hindex]
      wp_store
      wp_auto
      ihave Hlimbs := Hrestore $$ %item.value Hword
      isimp only [Int.toNat_natCast, paddedLimbs_step done item hsmall] at Hlimbs
      ieval (change (c ↦ {v with pos' := pos + W64 item.bytes.length, depth' := v.depth' + W64 1} : IProp GF) ⊢ _) at Hc
      dsimp only at Hnext
      have hbyteslen : (unsignedBytes (done ++ [item])).length = (unsignedBytes done).length + item.bytes.length := by simp [unsignedBytes]
      have hinc : W64 (done.length + passed.length) + W64 1 = W64 ((done ++ [item]).length + passed.length) := by
        simp only [List.length_append, List.length_singleton]; word
      wp_for_post
      isimp only [hinc] at Hrange
      iframe
      iexists (done ++ [item]), todo, passed, pending, (pos + W64 item.bytes.length), (W64 (done.length + passed.length))
      iframe
      ipureintro
      refine ⟨?_, Hextras, ?_, ⟨?_, Hnext.2.1⟩, ?_, Hnext.2.2⟩
      · simpa only [List.append_assoc, List.singleton_append] using Hsplit
      · intro _; exact hpassed
      · rw [Hnext.1]; omega
      · rw [Hnext.1, hbyteslen]; omega

/-- Any array with an extra element necessarily bypasses the four-limb shortcuts.
    Compose the actual ignored-element failure without publishing a partial felt. -/
theorem wp_readFelt_extras_reject (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (count : w64)
    (items : List UnsignedWire) (extras : List (List w8))
    (dq : DFrac) (R : IProp GF) (E : Loc → IProp GF)
    (d : Nat) (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hcount : items.length = 4) (hmore : 4 + extras.length < uint.Z count)
    (valid : ∀ item ∈ extras, SkipValid (d + 1) item)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ extras.flatten ++ rem)
    (hbad :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length), depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
      {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
         c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length), depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs ∗ E err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       R ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  wp_start as ⟨HR, Hdst, Hc, Hs⟩
  have hi := input_suffix_head xs (payload ++ unsignedBytes items ++ extras.flatten ++ rem)
    (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hn : head ≠ W8 246 := by intro h; subst head; contradiction
  have hu : head ≠ W8 247 := by intro h; subst head; contradiction
  have hne : head ≠ W8 132 := by
    intro heq
    obtain ⟨_, hc⟩ := headArgument_inline_inv head payload count enc (by rw [heq]; decide)
    rw [heq] at hc
    have hv : count = W64 4 := hc
    rw [hv] at hmore
    change (4 : Int) + extras.length < 4 at hmore
    omega
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_limbs_nonfast_reject c «$r0_ptr» (zero_val (GoArray w64 4)) v xs
    (payload ++ unsignedBytes items ++ extras.flatten ++ rem) head dq R E hne hp (by omega) hbytes
    (wp_limbsSlow_extras_reject c «$r0_ptr» (zero_val (GoArray w64 4)) v xs payload rem head count items extras
      dq R E d hd hdepth enc hm hcount hmore valid hp hbytes hbad)
    $$ [HR «$r0» Hc Hs] as %err ⟨Hv, Hc, Hs, Herr⟩
  · iframe HR «$r0» Hc Hs
  wp_end

/-- EOF in the ignored tail rolls back the whole felt while retaining the EOF offset. -/
theorem wp_readFelt_extras_eof (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload : List w8) (head : w8) (count : w64) (items : List UnsignedWire)
    (extras : List (List w8)) (dq : DFrac) (kind : GoInterface)
    (d : Nat) (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hcount : items.length = 4) (hmore : 4 + extras.length < uint.Z count)
    (valid : ∀ item ∈ extras, SkipValid (d + 1) item)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ extras.flatten) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.buf'.len (go!"input ends before the item") slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  let pos := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length)
  have heof : pos = v.buf'.len := by
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
      List.length_append, List.length_cons] at hlen
    dsimp only [pos]
    simp only [sint.nat] at hlen Hlen
    word
  have hleaf :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         globalAddr cursor.ErrTruncated ↦□ kind ∗
         c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
      {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
         c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
         err ↦ cursor.DecodeError.mk kind v.buf'.len (go!"input ends before the item") slice.nil }} := by
    wp_start_folded as ⟨#Hg, Hcc, Hss⟩
    wp_apply wp_Skip_eof c {v with pos' := pos, depth' := v.depth' + W64 1} kind heof Hwf
      $$ [Hcc] as %err ⟨Hcc, Herr⟩
    · iframe Hg Hcc
    isimp only [heof] at Herr
    wp_end
  wp_apply wp_readFelt_extras_reject c dst old v xs payload [] head count items extras dq
    iprop(globalAddr cursor.ErrTruncated ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.buf'.len (go!"input ends before the item") slice.nil))
    d hd hdepth enc hm hcount hmore valid hp (by simpa only [List.append_nil] using hbytes) hleaf
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end

/-- A rejected atomic extra after arbitrary valid skipped contents preserves the
    outer felt and input. The diagnostic retains Skip's post-head offset. -/
theorem wp_readFelt_extras_atomic (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload badpayload rem : List w8) (head bad : w8) (count argument : w64)
    (items : List UnsignedWire) (extras : List (List w8)) (dq : DFrac)
    (reason : SkipAtomError) (kind : GoInterface)
    (d : Nat) (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hcount : items.length = 4) (hmore : 4 + extras.length < uint.Z count)
    (valid : ∀ item ∈ extras, SkipValid (d + 1) item)
    (badenc : HeadArgument bad badpayload argument) (reject : SkipReject bad argument rem reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ extras.flatten ++ ((bad :: badpayload) ++ rem)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       skipAtomErrorAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind
         (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length) + W64 (1 + badpayload.length))
         (skipAtomErrorMessage reason) slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  let pos := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length)
  have Hnext := input_consumption xs ((head :: payload) ++ unsignedBytes items ++ extras.flatten)
    ((bad :: badpayload) ++ rem) v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpnext : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    dsimp only [pos]; constructor <;> word
  have hsuffix : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = (bad :: badpayload) ++ rem := by
    simpa only [pos, List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm, Int.add_left_comm, Int.add_assoc] using Hnext.2.2
  wp_apply wp_readFelt_extras_reject c dst old v xs payload ((bad :: badpayload) ++ rem) head count items extras dq
    iprop(skipAtomErrorAddr (GF := GF) reason ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind (pos + W64 (1 + badpayload.length))
      (skipAtomErrorMessage reason) slice.nil))
    d hd hdepth enc hm hcount hmore valid hp hbytes
    (wp_Skip_reject c {v with pos' := pos, depth' := v.depth' + W64 1} xs badpayload rem bad argument dq
      reason kind badenc reject hpnext hsuffix)
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbsSlow_extras_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_extras_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_extras_eof
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_extras_atomic
