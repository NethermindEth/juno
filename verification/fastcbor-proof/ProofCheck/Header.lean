import ProofCheck.InitializationState

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 1600000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp]
variable [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The generated zero check accepts the zero Header without modifying it. -/
theorem wp_zeroHeader (dst : Loc) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       dst ↦ (zero_val Header) }}
      (App (Val (@! cborZeroHeader)) (Val #dst))
    {{ RET #true; dst ↦ (zero_val Header) }} := by
  wp_start as Hdst
  wp_auto
  wp_end

/-- Reject a prefilled header without touching its other fields or its input. -/
theorem wp_nonzeroHeader (dst : Loc) (h : Header) (hne : h.Hash' ≠ null) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗ dst ↦ h }}
      (App (Val (@! cborZeroHeader)) (Val #dst))
    {{ RET #false; dst ↦ h }} := by
  wp_start as Hdst
  wp_auto
  simp only [hne, decide_false]
  wp_auto
  wp_end



theorem wp_newCursor (s : GoSlice) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! github_com.NethermindEth.juno.utils.cbor.cursor.NewCursor)) (Val #s))
    {{ RET #({ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) with buf' := s }); True }} := by
  wp_start
  wp_auto
  wp_end

/-- Actual Null implementation on a nonempty null item, preserving the bytes. -/
theorem wp_null (c : Loc) (s : GoSlice) (tail : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ ({ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) with buf' := s }) ∗ s ↦*{dq} (W8 246 :: tail) }}
      (App (Val (c @!! go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.Cursor.ty @!! go!"Null")) (Val #()))
    {{ RET #true;
       c ↦ ({ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) with buf' := s, pos' := W64 1 }) ∗
       s ↦*{dq} (W8 246 :: tail) }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  wp_auto
  have hlt : sint.Z (0 : w64) < sint.Z s.len := by
    simp only [List.length_cons, sint.nat, sint.Z] at Hlen ⊢
    change 0 < s.len.toInt
    omega
  simp only [hlt, _root_.decide_true]
  wp_auto
  have hindex : 0 ≤ sint.Z (0 : w64) ∧ sint.Z (0 : w64) < sint.Z s.len := ⟨by decide, hlt⟩
  rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
  wp_apply wp_load_slice_index s _ _ dq (W8 246) (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; rfl
  wp_auto
  wp_end

/-- Opening a null struct consumes only the null item. -/
theorem wp_openStruct_null (c fields : Loc) (s : GoSlice) (tail : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ ({ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) with buf' := s }) ∗ s ↦*{dq} (W8 246 :: tail) }}
      (App (Val (c @!! go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.Cursor.ty @!! go!"OpenStruct")) (Val #fields))
    {{ RET (PairV #({ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Struct) with fields' := fields }) #interface.nil);
       c ↦ ({ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) with buf' := s, pos' := W64 1 }) ∗
       s ↦*{dq} (W8 246 :: tail) }} := by
  wp_start as ⟨Hc, Hs⟩
  wp_auto
  wp_apply wp_null c s tail dq $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

/-- A closed, empty struct has no next field and touches no cursor memory. -/
theorem wp_nextField_slow_done (c st fields : Loc) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       st ↦ ({ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Struct) with fields' := fields }) }}
      (App (Val (c @!! go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.Cursor.ty @!! go!"nextField")) (Val #st))
    {{ RET (PairV #(W64 (-1)) #interface.nil);
       st ↦ ({ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Struct) with fields' := fields }) }} := by
  wp_start as Hst
  wp_auto
  simp only [Bool.false_eq_true, if_false]
  wp_auto
  wp_end

/-- A closed, empty struct has no next field and touches no cursor memory. -/
theorem wp_nextField_done (c st fields : Loc) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       st ↦ ({ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Struct) with fields' := fields }) }}
      (App (Val (c @!! go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ RET (PairV #(W64 (-1)) #interface.nil);
       st ↦ ({ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Struct) with fields' := fields }) }} := by
  wp_start as Hst
  wp_auto
  wp_apply wp_nextField_slow_done c st fields $$ [Hst] with Hst
  · iframe Hst
  wp_end

/-- The actual generated header loop accepts null, without modifying the header. -/
theorem wp_readHeader_null (c dst fields : Loc) (h : Header) (s : GoSlice)
    (tail : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ dst ↦ h ∗
       c ↦ ({ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) with buf' := s }) ∗ s ↦*{dq} (W8 246 :: tail) }}
      (App (App (Val (@! cborReadHeader)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ h ∗
       c ↦ ({ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) with buf' := s, pos' := W64 1 }) ∗
       s ↦*{dq} (W8 246 :: tail) }} := by
  wp_start as ⟨#Hfields, Hdst, Hc, Hs⟩
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_null c fields s tail dq $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_for
  wp_apply wp_nextField_done c _ fields $$ [s] with Hst
  · iframe
  wp_for_post
  wp_end

/-- The suffix operation neither copies nor writes the original bytes. -/
theorem wp_remaining (c : Loc) (v : github_com.NethermindEth.juno.utils.cbor.cursor.Cursor)
    (hpos : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hcap : sint.Z v.buf'.len ≤ sint.Z v.buf'.cap) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗ c ↦ v }}
      (App (Val (c @!! go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.Cursor.ty @!! go!"Remaining")) (Val #()))
    {{ RET #(slice.slice v.buf' w8 v.pos' v.buf'.len); c ↦ v }} := by
  wp_start as Hc
  wp_auto
  simp only [hpos.1, hpos.2, hcap, and_self, if_true]
  wp_auto
  wp_end

/-- Null succeeds through the public typed entry, with the exact input suffix. -/
theorem wp_decodeHeaderFirst_null (dst fields : Loc) (s : GoSlice)
    (tail : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ dst ↦ (zero_val Header) ∗
       s ↦*{dq} (W8 246 :: tail) }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #s)) (Val #dst))
    {{ RET (PairV #(slice.slice s w8 (W64 1) s.len) #interface.nil);
       dst ↦ (zero_val Header) ∗ s ↦*{dq} (W8 246 :: tail) }} := by
  wp_start as ⟨#Hfields, Hdst, Hs⟩
  wp_auto
  wp_apply wp_zeroHeader dst $$ [Hdst] with Hdst
  · iframe Hdst
  wp_apply wp_newCursor s
  wp_apply wp_readHeader_null _ dst fields (zero_val Header) s tail dq $$ [c Hdst Hs] with ⟨Hdst, Hc, Hs⟩
  · iframe Hdst Hs Hfields c
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hcap := ownSlice_wf _ _ _ $$ Hs
  wp_apply wp_remaining _ ({ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) with buf' := s, pos' := W64 1 }) (by simp only [List.length_cons, sint.nat, sint.Z] at Hlen ⊢; change 0 ≤ (1 : Int) ∧ 1 ≤ s.len.toInt; omega) Hcap.2 $$ [Hc] with Hc
  · iframe Hc
  wp_end

/-- A prefilled header is rejected without reading the input or modifying the destination. -/
theorem wp_decodeHeaderFirst_nonzero (dst : Loc) (h : Header) (s : GoSlice)
    (kind : GoInterface) (hne : h.Hash' ≠ null) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr github_com.NethermindEth.juno.utils.cbor.fastcbor.ErrUnsupported ↦□ kind ∗ dst ↦ h }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #s)) (Val #dst))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType github_com.NethermindEth.juno.utils.cbor.fastcbor.DecodeError.ty) #err));
       dst ↦ h ∗ err ↦ ({ (zero_val github_com.NethermindEth.juno.utils.cbor.fastcbor.DecodeError) with Kind' := kind, Detail' := go!"the destination is not the zero value" }) }} := by
  wp_start as ⟨#Hkind, Hdst⟩
  wp_auto
  wp_apply wp_nonzeroHeader dst h hne $$ [Hdst] with Hdst
  · iframe Hdst
  wp_alloc err as Herr
  wp_auto
  wp_end

/-- The actual cleanup operation clears every field, including a partial result. -/
theorem wp_clearHeader (dst : Loc) (h : Header) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗ dst ↦ h }}
      (App (Val (@! cborClearHeader)) (Val #dst))
    {{ RET #(); dst ↦ (zero_val Header) }} := by
  wp_start as Hdst
  wp_auto
  wp_end

/-- The full-input entry rejects a null item with trailing data and clears dst. -/
theorem wp_decodeHeader_trailing (dst fields : Loc) (s : GoSlice)
    (tail : List w8) (dq : DFrac) (kind : GoInterface) (hmore : tail ≠ []) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗
       globalAddr github_com.NethermindEth.juno.utils.cbor.fastcbor.ErrExtraneousData ↦□ kind ∗ dst ↦ (zero_val Header) ∗
       s ↦*{dq} (W8 246 :: tail) }}
      (App (App (Val (@! DecodeHeader)) (Val #s)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType github_com.NethermindEth.juno.utils.cbor.fastcbor.DecodeError.ty) #err);
       dst ↦ (zero_val Header) ∗ s ↦*{dq} (W8 246 :: tail) ∗
       err ↦ ({ (zero_val github_com.NethermindEth.juno.utils.cbor.fastcbor.DecodeError) with Kind' := kind, Extra' := s.len - W64 1 }) }} := by
  wp_start as ⟨#Hfields, #Hkind, Hdst, Hs⟩
  wp_auto
  wp_apply wp_decodeHeaderFirst_null dst fields s tail dq $$ [Hdst Hs] with ⟨Hdst, Hs⟩
  · iframe Hdst Hs Hfields
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hne : s.len - W64 1 ≠ W64 0 := by
    simp only [List.length_cons, sint.nat, sint.Z] at Hlen
    have : 0 < tail.length := List.length_pos_iff.mpr hmore
    word
  simp only [slice.slice, hne, decide_false]
  wp_auto
  wp_apply wp_clearHeader dst (zero_val Header) $$ [Hdst] with Hdst
  · iframe Hdst
  wp_alloc err as Herr
  wp_auto
  wp_end

/-- Error construction preserves the category and cursor position without formatting. -/
theorem wp_fail (c : Loc) (v : github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) (kind : GoInterface)
    (reason : GoString) (args : GoSlice) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗ c ↦ v }}
      (App (App (App (Val (c @!! go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.Cursor.ty @!! go!"fail")) (Val #kind)) (Val #reason)) (Val #args))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.DecodeError.ty) #err);
       c ↦ v ∗ err ↦ (github_com.NethermindEth.juno.utils.cbor.cursor.DecodeError.mk kind v.pos' reason args) }} := by
  wp_start as Hc
  wp_auto
  wp_alloc err as Herr
  wp_auto
  wp_end

/-- Empty input fails before consuming anything. -/
theorem wp_headSlow_empty (c : Loc) (kind : GoInterface) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr github_com.NethermindEth.juno.utils.cbor.cursor.ErrTruncated ↦□ kind ∗ c ↦ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) }}
      (App (Val (c @!! go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.Cursor.ty @!! go!"headSlow")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #(W8 0) #(W64 0)) #(interface.mkOk (go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.DecodeError.ty) #err));
       c ↦ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) ∗
       err ↦ (github_com.NethermindEth.juno.utils.cbor.cursor.DecodeError.mk kind (W64 0) (go!"input ends before the item") slice.nil) }} := by
  wp_start as ⟨#Hkind, Hc⟩
  wp_auto
  wp_apply wp_fail c (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) kind (go!"input ends before the item") slice.nil $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc
  wp_end

/-- Null does not advance a cursor over empty input. -/
theorem wp_null_empty (c : Loc) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗ c ↦ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) }}
      (App (Val (c @!! go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.Cursor.ty @!! go!"Null")) (Val #()))
    {{ RET #false; c ↦ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) }} := by
  wp_start as Hc
  wp_auto
  wp_end

theorem wp_expectSlow_empty (c : Loc) (kind : GoInterface) (want : w8) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr github_com.NethermindEth.juno.utils.cbor.cursor.ErrTruncated ↦□ kind ∗ c ↦ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) }}
      (App (Val (c @!! go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.Cursor.ty @!! go!"expectSlow")) (Val #want))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.DecodeError.ty) #err));
       c ↦ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) ∗
       err ↦ (github_com.NethermindEth.juno.utils.cbor.cursor.DecodeError.mk kind (W64 0) (go!"input ends before the item") slice.nil) }} := by
  wp_start as ⟨#Hkind, Hc⟩
  wp_auto
  wp_apply wp_headSlow_empty c kind $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc Hkind
  wp_end

theorem wp_containerLen_empty (c : Loc) (kind : GoInterface) (want : w8) (perItem : w64) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr github_com.NethermindEth.juno.utils.cbor.cursor.ErrTruncated ↦□ kind ∗ c ↦ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) }}
      (App (App (Val (c @!! go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.Cursor.ty @!! go!"containerLen")) (Val #want)) (Val #perItem))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.DecodeError.ty) #err));
       c ↦ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) ∗
       err ↦ (github_com.NethermindEth.juno.utils.cbor.cursor.DecodeError.mk kind (W64 0) (go!"input ends before the item") slice.nil) }} := by
  wp_start as ⟨#Hkind, Hc⟩
  wp_auto
  wp_apply wp_expectSlow_empty c kind want $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc Hkind
  wp_end

theorem wp_mapLen_empty (c : Loc) (kind : GoInterface)  :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr github_com.NethermindEth.juno.utils.cbor.cursor.ErrTruncated ↦□ kind ∗ c ↦ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) }}
      (App (Val (c @!! go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.Cursor.ty @!! go!"MapLen")) (Val #()))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.DecodeError.ty) #err));
       c ↦ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) ∗
       err ↦ (github_com.NethermindEth.juno.utils.cbor.cursor.DecodeError.mk kind (W64 0) (go!"input ends before the item") slice.nil) }} := by
  wp_start as ⟨#Hkind, Hc⟩
  wp_auto
  wp_apply wp_containerLen_empty c kind (W8 160) (W64 2) $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc Hkind
  wp_end

theorem wp_openStruct_empty (c fields : Loc) (kind : GoInterface) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr github_com.NethermindEth.juno.utils.cbor.cursor.ErrTruncated ↦□ kind ∗ c ↦ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) }}
      (App (Val (c @!! go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.Cursor.ty @!! go!"OpenStruct")) (Val #fields))
    {{ (err : Loc), RET (PairV #({ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Struct) with fields' := fields }) #(interface.mkOk (go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.DecodeError.ty) #err));
       c ↦ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) ∗
       err ↦ (github_com.NethermindEth.juno.utils.cbor.cursor.DecodeError.mk kind (W64 0) (go!"input ends before the item") slice.nil) }} := by
  wp_start as ⟨#Hkind, Hc⟩
  wp_auto
  wp_apply wp_null_empty c $$ [Hc] with Hc
  · iframe Hc
  wp_apply wp_mapLen_empty c kind $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc Hkind
  wp_end

/-- A parse error before the map reaches the generated reader unchanged. -/
theorem wp_readHeader_empty (c dst fields : Loc) (h : Header) (kind : GoInterface) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ globalAddr github_com.NethermindEth.juno.utils.cbor.cursor.ErrTruncated ↦□ kind ∗
       dst ↦ h ∗ c ↦ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) }}
      (App (App (Val (@! cborReadHeader)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.DecodeError.ty) #err);
       dst ↦ h ∗ c ↦ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) ∗
       err ↦ (github_com.NethermindEth.juno.utils.cbor.cursor.DecodeError.mk kind (W64 0) (go!"input ends before the item") slice.nil) }} := by
  wp_start as ⟨#Hfields, #Hkind, Hdst, Hc⟩
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_empty c fields kind $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc Hkind
  wp_for
  wp_end

/-- Public entry propagates the parse error and returns a cleared destination. -/
theorem wp_decodeHeaderFirst_empty (dst fields : Loc) (kind : GoInterface) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ globalAddr github_com.NethermindEth.juno.utils.cbor.cursor.ErrTruncated ↦□ kind ∗
       dst ↦ (zero_val Header) }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #slice.nil)) (Val #dst))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.DecodeError.ty) #err));
       dst ↦ (zero_val Header) ∗
       err ↦ (github_com.NethermindEth.juno.utils.cbor.cursor.DecodeError.mk kind (W64 0) (go!"input ends before the item") slice.nil) }} := by
  wp_start as ⟨#Hfields, #Hkind, Hdst⟩
  wp_auto
  wp_apply wp_zeroHeader dst $$ [Hdst] with Hdst
  · iframe Hdst
  wp_apply wp_newCursor slice.nil
  wp_apply wp_readHeader_empty _ dst fields (zero_val Header) kind $$ [c Hdst] as %err ⟨Hdst, Hc, Herr⟩
  · iframe c Hdst Hfields Hkind
  wp_apply wp_clearHeader dst (zero_val Header) $$ [Hdst] with Hdst
  · iframe Hdst
  wp_end

/-- All 24 inline unsigned integers, with an arbitrary suffix. -/
theorem wp_expect_inline (c : Loc) (s : GoSlice) (b : w8) (tail : List w8)
    (dq : DFrac) (hb : uint.Z b < 24) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ ({ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) with buf' := s }) ∗ s ↦*{dq} (b :: tail) }}
      (App (Val (c @!! go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.Cursor.ty @!! go!"expect")) (Val #(W8 0)))
    {{ RET (PairV #(W64 (uint.Z b)) #interface.nil);
       c ↦ ({ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) with buf' := s, pos' := W64 1 }) ∗ s ↦*{dq} (b :: tail) }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  wp_auto
  have hlt : sint.Z (0 : w64) < sint.Z s.len := by
    simp only [List.length_cons, sint.nat, sint.Z] at Hlen ⊢
    change 0 < s.len.toInt
    omega
  simp only [hlt, decide_true]
  wp_auto
  have hindex : 0 ≤ sint.Z (0 : w64) ∧ sint.Z (0 : w64) < sint.Z s.len := ⟨by decide, hlt⟩
  rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
  wp_apply wp_load_slice_index s _ _ dq b (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; rfl
  have hb' : uint.Z (b - W8 0) < uint.Z (W8 24) := by word
  simp only [hb', _root_.decide_true]
  wp_auto
  have hsub : b - W8 0 = b := by word
  simp only [hsub]
  wp_end

theorem wp_uint64_inline (c : Loc) (s : GoSlice) (b : w8) (tail : List w8)
    (dq : DFrac) (hb : uint.Z b < 24) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ ({ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) with buf' := s }) ∗ s ↦*{dq} (b :: tail) }}
      (App (Val (c @!! go.PointerType github_com.NethermindEth.juno.utils.cbor.cursor.Cursor.ty @!! go!"Uint64")) (Val #()))
    {{ RET (PairV #(W64 (uint.Z b)) #interface.nil);
       c ↦ ({ (zero_val github_com.NethermindEth.juno.utils.cbor.cursor.Cursor) with buf' := s, pos' := W64 1 }) ∗ s ↦*{dq} (b :: tail) }} := by
  wp_start as ⟨Hc, Hs⟩
  wp_auto
  wp_apply wp_expect_inline c s b tail dq hb $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_zeroHeader
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nonzeroHeader
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_newCursor
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_null
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_openStruct_null
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_slow_done
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_done
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readHeader_null
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_remaining
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_null
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_nonzero
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_trailing
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_fail
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headSlow_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_null_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_expectSlow_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_containerLen_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_mapLen_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_openStruct_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readHeader_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_empty

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_expect_inline
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_uint64_inline

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_clearHeader
