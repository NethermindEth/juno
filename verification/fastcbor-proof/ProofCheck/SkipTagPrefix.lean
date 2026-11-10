import ProofCheck.SkipTagReject

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

/-- Supported tag prefixes before an arbitrary remaining input. Each immediate
    tagged child charges a level; a tag before another major type is free. -/
inductive SkipTagPrefix (tail : List w8) : Nat → List w8 → Nat → Prop where
  | nil (d : Nat) : SkipTagPrefix tail d [] d
  | tag {d leaf : Nat} {head next : w8} {payload front rest : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 192)
      (number : 3 < uint.Z argument) (nonempty : front ++ tail = next :: rest)
      (bound : next &&& W8 224 = W8 192 → d < 32)
      (child : SkipTagPrefix tail (d + if next &&& W8 224 = W8 192 then 1 else 0) front leaf) :
      SkipTagPrefix tail d ((head :: payload) ++ front) leaf

/-- Arbitrary tag prefixes preserve a proved leaf failure, its owned diagnostic
    and the worker's final cursor. All intermediate recursive calls are proved. -/
theorem wp_skip_tag_prefix_reject (c : Loc) (v : cursor.Cursor)
    (xs front tail : List w8) (dq : DFrac) (depth out : w64) (d leaf : Nat)
    (R : IProp GF) (E : Loc → IProp GF)
    (tags : SkipTagPrefix tail d front leaf) (hd : sint.Z depth = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front ++ tail)
    (hbad :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ {v with pos' := v.pos' + W64 front.length} ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #(W64 leaf)))
      {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
         c ↦ {v with pos' := out} ∗ v.buf' ↦*{dq} xs ∗ E err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ {v with pos' := out} ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  induction tags generalizing v depth with
  | nil d =>
    have hz : v.pos' + W64 ([] : List w8).length = v.pos' := by simp only [List.length_nil]; word
    have hdep : W64 d = depth := by word
    simpa only [hz, hdep] using hbad
  | @tag d leaf head next payload front rest argument enc hm number nonempty bound child ih =>
    wp_start_folded as ⟨HR, Hc, Hs⟩
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have Hnext := input_consumption xs (head :: payload) (front ++ tail) v.pos' v.buf'.len hp Hlen.1
      (by simpa only [List.append_assoc] using hbytes)
    let pos := v.pos' + W64 (1 + payload.length)
    have hpos : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len := by
      simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
      dsimp only [pos]; constructor <;> word
    have hchild : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = front ++ tail := by
      simpa only [pos, List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hnext.2.2
    let childDepth := depth + W64 (if next &&& W8 224 = W8 192 then 1 else 0)
    have hdc : sint.Z childDepth = (d + if next &&& W8 224 = W8 192 then 1 else 0 : Nat) := by
      dsimp only [childDepth]
      by_cases ht : next &&& W8 224 = W8 192 <;> simp only [ht, ite_true, ite_false] <;> word
    have hbd : (d + if next &&& W8 224 = W8 192 then 1 else 0 : Nat) ≤ 32 := by
      by_cases ht : next &&& W8 224 = W8 192
      · have := bound ht; simp only [ht, ite_true]; omega
      · simp only [ht, ite_false, Nat.add_zero]; exact hdepth
    have hsum : pos + W64 front.length = v.pos' + W64 ((head :: payload) ++ front).length := by
      simp only [List.length_append, List.length_cons]
      dsimp only [pos]; word
    have childSpec := ih {v with pos' := pos} childDepth hdc hbd hpos hchild
      (by simpa only [hsum] using hbad)
    wp_apply wp_skip_tag_reject c v xs payload rest [] head next argument depth out dq R E enc hm number
      ⟨by omega, by omega⟩ (by intro ht; have := bound ht; omega) childSpec hp
      (by simpa only [List.append_nil, List.append_assoc, nonempty] using hbytes)
      $$ [HR Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe HR Hc Hs
    wp_end

/-- Closed worker contract for any supported tag chain ending at a malformed
    head. The leaf is discharged by the actual head-error proof. -/
theorem wp_skip_tagged_fault (c : Loc) (v : cursor.Cursor)
    (xs front tail : List w8) (bad : w8) (dq : DFrac) (depth : w64) (d leaf : Nat)
    (reason : HeadError) (kind : GoInterface)
    (tags : SkipTagPrefix (bad :: tail) d front leaf) (fault : HeadFault bad reason)
    (hd : sint.Z depth = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front ++ (bad :: tail)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       headFaultAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ {v with pos' := v.pos' + W64 front.length} ∗ v.buf' ↦*{dq} xs ∗
       HeadFaultError err (v.pos' + W64 front.length) bad reason kind }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs front (bad :: tail) v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpos : 0 ≤ sint.Z (v.pos' + W64 front.length) ∧
      sint.Z (v.pos' + W64 front.length) ≤ sint.Z v.buf'.len := by constructor <;> word
  wp_apply wp_skip_tag_prefix_reject c v xs front (bad :: tail) dq depth
    (v.pos' + W64 front.length) d leaf iprop(headFaultAddr (GF := GF) reason ↦□ kind)
    (fun err => HeadFaultError err (v.pos' + W64 front.length) bad reason kind)
    tags hd hdepth hp hbytes
    (wp_skip_fault c {v with pos' := v.pos' + W64 front.length} (W64 leaf) xs tail bad dq reason kind fault hpos Hnext.2.2)
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Public Skip restores the entire original cursor while retaining a tagged
    malformed head's inner offset and exact owned diagnostic. -/
theorem wp_Skip_tagged_fault (c : Loc) (v : cursor.Cursor)
    (xs front tail : List w8) (bad : w8) (dq : DFrac) (d leaf : Nat)
    (reason : HeadError) (kind : GoInterface)
    (tags : SkipTagPrefix (bad :: tail) d front leaf) (fault : HeadFault bad reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front ++ (bad :: tail)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       headFaultAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ HeadFaultError err (v.pos' + W64 front.length) bad reason kind }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_skip_tagged_fault c v xs front tail bad dq v.depth' d leaf reason kind tags fault hd hdepth hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_tag_prefix_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_tagged_fault
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_tagged_fault
