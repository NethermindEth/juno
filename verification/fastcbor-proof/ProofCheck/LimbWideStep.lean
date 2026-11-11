import ProofCheck.LimbWideSpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 4096

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- The translated readLimbs fast branch. Composition with the actual loop must
    match this expression; it is not an assumed implementation contract. -/
def limbWideStepExpr (bufLoc feltsLoc iLoc : Loc) : Expr :=
(Let "$r0" (App (Val (GoInstruction (IndexRef (go.GoType.SliceType (go.GoType.ArrayType 4 go.uint64))))) (Pair (App (Val (GoInstruction (GoLoad (go.GoType.SliceType (go.GoType.ArrayType 4 go.uint64))))) (Val #feltsLoc)) (App (Val (GoInstruction (GoLoad go.int))) (Val #iLoc))))
  (Let "felt" (App (Val (GoInstruction (GoAlloc (go.GoType.PointerType (go.GoType.ArrayType 4 go.uint64))))) (App (Val (GoInstruction (GoZeroVal (go.GoType.PointerType (go.GoType.ArrayType 4 go.uint64))))) (Val #())))
  (App (App (Val exceptionSeq) (Lam BAnon
  (Let "$r0" (Let "$a0" (Let "$s" (App (Val (GoInstruction (GoLoad (go.GoType.SliceType go.byte)))) (Val #bufLoc))
  (App (Val (GoInstruction (Slice (go.GoType.SliceType go.byte)))) (Pair (Pair (Var "$s") (Val #(W64 2))) (App (App (Val (GoInstruction (FuncResolve go.len [(go.GoType.SliceType go.byte)]))) (Val #())) (Var "$s")))))
  (App (App (Val (GoInstruction (MethodResolve (go.GoType.PointerType _root_.Perennial.encoding.binary.bigEndian.ty) go!"Uint64"))) (App (Val (GoInstruction (GlobalVarAddr _root_.Perennial.encoding.binary.BigEndian))) (Val #()))) (Var "$a0")))
  (App (App (Val exceptionSeq) (Lam BAnon
  (Let "$r0" (Let "$a0" (Let "$s" (App (Val (GoInstruction (GoLoad (go.GoType.SliceType go.byte)))) (Val #bufLoc))
  (App (Val (GoInstruction (Slice (go.GoType.SliceType go.byte)))) (Pair (Pair (Var "$s") (Val #(W64 11))) (App (App (Val (GoInstruction (FuncResolve go.len [(go.GoType.SliceType go.byte)]))) (Val #())) (Var "$s")))))
  (App (App (Val (GoInstruction (MethodResolve (go.GoType.PointerType _root_.Perennial.encoding.binary.bigEndian.ty) go!"Uint64"))) (App (Val (GoInstruction (GlobalVarAddr _root_.Perennial.encoding.binary.BigEndian))) (Val #()))) (Var "$a0")))
  (App (App (Val exceptionSeq) (Lam BAnon
  (Let "$r0" (Let "$a0" (Let "$s" (App (Val (GoInstruction (GoLoad (go.GoType.SliceType go.byte)))) (Val #bufLoc))
  (App (Val (GoInstruction (Slice (go.GoType.SliceType go.byte)))) (Pair (Pair (Var "$s") (Val #(W64 20))) (App (App (Val (GoInstruction (FuncResolve go.len [(go.GoType.SliceType go.byte)]))) (Val #())) (Var "$s")))))
  (App (App (Val (GoInstruction (MethodResolve (go.GoType.PointerType _root_.Perennial.encoding.binary.bigEndian.ty) go!"Uint64"))) (App (Val (GoInstruction (GlobalVarAddr _root_.Perennial.encoding.binary.BigEndian))) (Val #()))) (Var "$a0")))
  (App (App (Val exceptionSeq) (Lam BAnon
  (Let "$r0" (Let "$a0" (Let "$s" (App (Val (GoInstruction (GoLoad (go.GoType.SliceType go.byte)))) (Val #bufLoc))
  (App (Val (GoInstruction (Slice (go.GoType.SliceType go.byte)))) (Pair (Pair (Var "$s") (Val #(W64 29))) (App (App (Val (GoInstruction (FuncResolve go.len [(go.GoType.SliceType go.byte)]))) (Val #())) (Var "$s")))))
  (App (App (Val (GoInstruction (MethodResolve (go.GoType.PointerType _root_.Perennial.encoding.binary.bigEndian.ty) go!"Uint64"))) (App (Val (GoInstruction (GlobalVarAddr _root_.Perennial.encoding.binary.BigEndian))) (Val #()))) (Var "$a0")))
  (App (App (Val exceptionSeq) (Lam BAnon
  (Let "$r0" (Let "$s" (App (Val (GoInstruction (GoLoad (go.GoType.SliceType go.byte)))) (Val #bufLoc))
  (App (Val (GoInstruction (Slice (go.GoType.SliceType go.byte)))) (Pair (Pair (Var "$s") (App (Val (GoInstruction (Convert go.untypedInt go.int))) (Val cursor.feltLen))) (App (App (Val (GoInstruction (FuncResolve go.len [(go.GoType.SliceType go.byte)]))) (Val #())) (Var "$s")))))
  (App (App (Val exceptionSeq) (Lam BAnon
  (App (Val doContinue) (Val #()))))
  (App (Val doExecute)
  (App (Val (GoInstruction (GoStore (go.GoType.SliceType go.byte)))) (Pair (Val #bufLoc) (Var "$r0"))))))))
  (App (Val doExecute)
  (App (Val (GoInstruction (GoStore go.uint64))) (Pair (App (Val (GoInstruction (IndexRef (go.GoType.ArrayType 4 go.uint64)))) (Pair (App (Val (GoInstruction (GoLoad (go.GoType.PointerType (go.GoType.ArrayType 4 go.uint64))))) (Var "felt")) (Val #(W64 3)))) (Var "$r0"))))))))
  (App (Val doExecute)
  (App (Val (GoInstruction (GoStore go.uint64))) (Pair (App (Val (GoInstruction (IndexRef (go.GoType.ArrayType 4 go.uint64)))) (Pair (App (Val (GoInstruction (GoLoad (go.GoType.PointerType (go.GoType.ArrayType 4 go.uint64))))) (Var "felt")) (Val #(W64 2)))) (Var "$r0"))))))))
  (App (Val doExecute)
  (App (Val (GoInstruction (GoStore go.uint64))) (Pair (App (Val (GoInstruction (IndexRef (go.GoType.ArrayType 4 go.uint64)))) (Pair (App (Val (GoInstruction (GoLoad (go.GoType.PointerType (go.GoType.ArrayType 4 go.uint64))))) (Var "felt")) (Val #(W64 1)))) (Var "$r0"))))))))
  (App (Val doExecute)
  (App (Val (GoInstruction (GoStore go.uint64))) (Pair (App (Val (GoInstruction (IndexRef (go.GoType.ArrayType 4 go.uint64)))) (Pair (App (Val (GoInstruction (GoLoad (go.GoType.PointerType (go.GoType.ArrayType 4 go.uint64))))) (Var "felt")) (Val #(W64 0)))) (Var "$r0"))))))))
  (App (Val doExecute)
  (App (Val (GoInstruction (GoStore (go.GoType.PointerType (go.GoType.ArrayType 4 go.uint64))))) (Pair (Var "felt") (Var "$r0")))))))

variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The fast branch writes exactly one owned slot, preserves all other slots
    and the input bytes, and advances only the local window by 37 bytes. -/
theorem wp_limbWideStep (bufLoc feltsLoc iLoc : Loc) (buf out : GoSlice)
    (values : List (GoArray w64 4)) (old : GoArray w64 4) (index : w64)
    (a b cc d rem : List w8) (dq : DFrac)
    (ha : a.length = 8) (hb : b.length = 8) (hc : cc.length = 8) (hd : d.length = 8)
    (hi : 0 ≤ sint.Z index) (lookup : values[(sint.Z index).toNat]? = some old) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       bufLoc ↦ buf ∗ feltsLoc ↦ out ∗ iLoc ↦ index ∗ out ↦* values ∗
       buf ↦*{dq} (wideFeltBytes a b cc d ++ rem) }}
      (limbWideStepExpr bufLoc feltsLoc iLoc)
    {{ RET continueVal;
       bufLoc ↦ slice.slice buf w8 (W64 37) buf.len ∗ feltsLoc ↦ out ∗ iLoc ↦ index ∗
       out ↦* values.set (sint.Z index).toNat (wideFeltValue a b cc d) ∗
       buf ↦*{dq} (wideFeltBytes a b cc d ++ rem) }} := by
  iintro %Φ ⟨#Hpkg, Hbufptr, Houtptr, Hiptr, Hvalues, Hbuf⟩ HΦ
  ihave %Houtlen := ownSlice_len _ _ _ $$ Hvalues
  have hidx : 0 ≤ sint.Z index ∧ sint.Z index < sint.Z out.len := by
    have hlt := (List.getElem?_eq_some_iff.mp lookup).choose
    simp only [sint.nat, sint.Z] at Houtlen hi hlt ⊢
    omega
  unfold limbWideStepExpr
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true hidx)]
  wp_auto
  let dst := sliceIndexRef (GoArray w64 4) (sint.Z index) out
  icases ownSlice_elem_acc (sint.Z index) old out (DFrac.own 1) values hi lookup $$ Hvalues
    with ⟨Hdst, Hclose⟩
  rcases old with ⟨old⟩
  ihave %Holdlen := array_len dst (DFrac.own 1) 4 old $$ Hdst
  have holdlen : old.length = 4 := by omega
  obtain ⟨o0, o1, o2, o3, rfl⟩ : ∃ o0 o1 o2 o3 : w64, old = [o0, o1, o2, o3] :=
    ⟨old[0]'(by omega), old[1]'(by omega), old[2]'(by omega), old[3]'(by omega),
      List.eq_getElem_of_length_eq_four old holdlen⟩
  ihave %Hbuflen := ownSlice_len _ _ _ $$ Hbuf
  ihave %Hbufwf := ownSlice_wf _ _ _ $$ Hbuf
  have hwide : (37 : Int) ≤ sint.Z buf.len := by
    have hlen := Hbuflen.1
    simp only [wideFeltBytes, List.length_append, List.length_cons, List.length_nil, ha, hb, hc, hd, sint.nat] at hlen
    simp only [sint.Z] at hlen ⊢
    omega
  have hfull : sint.nat buf.len = (wideFeltBytes a b cc d ++ rem).length := Hbuflen.1.symm
  have hbnd2 : 0 ≤ sint.Z (W64 2) ∧ sint.Z (W64 2) ≤ sint.Z buf.len ∧
      sint.Z buf.len ≤ sint.Z buf.cap := by
    exact ⟨by decide, by word, Hbufwf.2⟩
  rw [ite_eq_left_of_eq_true _ _ (eq_true hbnd2)]
  have hreadbytes : subslice 2 (sint.nat buf.len) (wideFeltBytes a b cc d ++ rem) =
      a ++ ([W8 27] ++ b ++ [W8 27] ++ cc ++ [W8 27] ++ d ++ rem) := by
    rw [hfull]
    simp only [subslice, List.take_length]
    simp [wideFeltBytes, List.append_assoc]
  wp_apply wp_bigEndian_offset buf
      (wideFeltBytes a b cc d ++ rem) a ([W8 27] ++ b ++ [W8 27] ++ cc ++ [W8 27] ++ d ++ rem)
      (W64 2) buf.len dq rfl ⟨hbnd2.1, hbnd2.2.1⟩ hreadbytes ha $$ [Hbuf] with Hbuf
  · iframe Hbuf
  icases array_acc dst 0 (DFrac.own 1) 4 (array.mk 4 [o0, o1, o2, o3]) o0
      (by decide) rfl $$ Hdst with ⟨Hword, Hrestore⟩
  simp only [show sint.Z (W64 0) = 0 from rfl]
  wp_store
  wp_auto
  ihave Hdst := Hrestore $$ %(leToU64 a.reverse) Hword
  ieval (change (dst ↦ array.mk 4 [leToU64 a.reverse, o1, o2, o3] : IProp GF) ⊢ _) at Hdst
  have hbnd11 : 0 ≤ sint.Z (W64 11) ∧ sint.Z (W64 11) ≤ sint.Z buf.len ∧
      sint.Z buf.len ≤ sint.Z buf.cap := ⟨by decide, by word, Hbufwf.2⟩
  rw [ite_eq_left_of_eq_true _ _ (eq_true hbnd11)]
  have hreadbytes11 : subslice 11 (sint.nat buf.len) (wideFeltBytes a b cc d ++ rem) =
      b ++ ([W8 27] ++ cc ++ [W8 27] ++ d ++ rem) := by
    rw [hfull]
    simp only [subslice, List.take_length]
    simp [wideFeltBytes, List.append_assoc, List.drop_append, List.drop_eq_nil_of_le, ha, hb, hc, hd]
  wp_apply wp_bigEndian_offset buf
      (wideFeltBytes a b cc d ++ rem) b ([W8 27] ++ cc ++ [W8 27] ++ d ++ rem)
      (W64 11) buf.len dq rfl ⟨hbnd11.1, hbnd11.2.1⟩ hreadbytes11 hb
      $$ [Hbuf] with Hbuf
  · iframe Hbuf
  icases array_acc dst 1 (DFrac.own 1) 4 (array.mk 4 [leToU64 a.reverse, o1, o2, o3]) o1
      (by decide) rfl $$ Hdst with ⟨Hword, Hrestore⟩
  simp only [show sint.Z (W64 1) = 1 from rfl]
  wp_store
  wp_auto
  ihave Hdst := Hrestore $$ %(leToU64 b.reverse) Hword
  ieval (change (dst ↦ array.mk 4 [leToU64 a.reverse, leToU64 b.reverse, o2, o3] : IProp GF) ⊢ _) at Hdst
  have hbnd20 : 0 ≤ sint.Z (W64 20) ∧ sint.Z (W64 20) ≤ sint.Z buf.len ∧
      sint.Z buf.len ≤ sint.Z buf.cap := ⟨by decide, by word, Hbufwf.2⟩
  rw [ite_eq_left_of_eq_true _ _ (eq_true hbnd20)]
  have hreadbytes20 : subslice 20 (sint.nat buf.len) (wideFeltBytes a b cc d ++ rem) =
      cc ++ ([W8 27] ++ d ++ rem) := by
    rw [hfull]
    simp only [subslice, List.take_length]
    simp [wideFeltBytes, List.append_assoc, List.drop_append, List.drop_eq_nil_of_le, ha, hb, hc, hd]
  wp_apply wp_bigEndian_offset buf
      (wideFeltBytes a b cc d ++ rem) cc ([W8 27] ++ d ++ rem)
      (W64 20) buf.len dq rfl ⟨hbnd20.1, hbnd20.2.1⟩ hreadbytes20 hc
      $$ [Hbuf] with Hbuf
  · iframe Hbuf
  icases array_acc dst 2 (DFrac.own 1) 4 (array.mk 4 [leToU64 a.reverse, leToU64 b.reverse, o2, o3]) o2
      (by decide) rfl $$ Hdst with ⟨Hword, Hrestore⟩
  simp only [show sint.Z (W64 2) = 2 from rfl]
  wp_store
  wp_auto
  ihave Hdst := Hrestore $$ %(leToU64 cc.reverse) Hword
  ieval (change (dst ↦ array.mk 4 [leToU64 a.reverse, leToU64 b.reverse, leToU64 cc.reverse, o3] : IProp GF) ⊢ _) at Hdst
  have hbnd29 : 0 ≤ sint.Z (W64 29) ∧ sint.Z (W64 29) ≤ sint.Z buf.len ∧
      sint.Z buf.len ≤ sint.Z buf.cap := ⟨by decide, by word, Hbufwf.2⟩
  rw [ite_eq_left_of_eq_true _ _ (eq_true hbnd29)]
  have hreadbytes29 : subslice 29 (sint.nat buf.len) (wideFeltBytes a b cc d ++ rem) =
      d ++ (rem) := by
    rw [hfull]
    simp only [subslice, List.take_length]
    simp [wideFeltBytes, List.append_assoc, List.drop_append, List.drop_eq_nil_of_le, ha, hb, hc, hd]
  wp_apply wp_bigEndian_offset buf
      (wideFeltBytes a b cc d ++ rem) d (rem)
      (W64 29) buf.len dq rfl ⟨hbnd29.1, hbnd29.2.1⟩ hreadbytes29 hd
      $$ [Hbuf] with Hbuf
  · iframe Hbuf
  icases array_acc dst 3 (DFrac.own 1) 4 (array.mk 4 [leToU64 a.reverse, leToU64 b.reverse, leToU64 cc.reverse, o3]) o3
      (by decide) rfl $$ Hdst with ⟨Hword, Hrestore⟩
  simp only [show sint.Z (W64 3) = 3 from rfl]
  wp_store
  wp_auto
  ihave Hdst := Hrestore $$ %(leToU64 d.reverse) Hword
  ieval (change (dst ↦ array.mk 4 [leToU64 a.reverse, leToU64 b.reverse, leToU64 cc.reverse, leToU64 d.reverse] : IProp GF) ⊢ _) at Hdst
  have hbnd37 : 0 ≤ sint.Z (W64 37) ∧ sint.Z (W64 37) ≤ sint.Z buf.len ∧
      sint.Z buf.len ≤ sint.Z buf.cap := ⟨by decide, hwide, Hbufwf.2⟩
  rw [ite_eq_left_of_eq_true _ _ (eq_true hbnd37)]
  wp_auto
  isimp only [dst] at Hdst
  ihave Hvalues := Hclose $$ %(array.mk 4 [leToU64 a.reverse, leToU64 b.reverse, leToU64 cc.reverse, leToU64 d.reverse]) Hdst
  isimp only [wideFeltValue] at HΦ
  iapply HΦ
  iframe Hbufptr Houtptr Hiptr Hvalues Hbuf

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbWideStep
