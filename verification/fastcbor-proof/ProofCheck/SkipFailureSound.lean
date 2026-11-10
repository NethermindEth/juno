import ProofCheck.SkipFailureSpec

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

/-- One public rejection contract for every specified Skip error class, reached
    through arbitrary supported enclosing contexts. All actual leaf and context
    proofs are instantiated; no helper-correctness assumptions remain. -/
theorem wp_Skip_failure (c : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (d offset : Nat) (reason : SkipFailureReason) (kind : GoInterface)
    (failure : SkipFailure d wire offset reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       skipFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ SkipFailureError err (v.pos' + W64 offset) reason kind }} := by
  cases failure with
  | @within front tail leaf offset reason context failure =>
    cases failure with
    | @head leaf bad tail reason fault =>
      simpa only [skipFailureAddr, SkipFailureError, Nat.add_zero] using
        wp_Skip_context_head (GF := GF) c v xs front tail bad dq d leaf reason kind context fault hd hdepth hp hbytes
    | @incomplete leaf head tail size info sizeEq short =>
      simpa only [skipFailureAddr, SkipFailureError, Nat.add_zero] using
        wp_Skip_context_truncated (GF := GF) c v xs front tail head size dq d leaf kind context info sizeEq short hd hdepth hp hbytes
    | @atomic leaf head payload tail argument reason enc reject =>
      have hpos : v.pos' + W64 (front.length + (1 + payload.length) : Nat) =
          v.pos' + W64 front.length + W64 (1 + payload.length) := by word
      simpa only [skipFailureAddr, SkipFailureError, hpos] using
        wp_Skip_context_atomic (GF := GF) c v xs front payload tail head argument dq d leaf reason kind context enc reject hd hdepth hp hbytes
    | eof leaf =>
      simpa only [skipFailureAddr, SkipFailureError, Nat.add_zero] using
        wp_Skip_context_eof (GF := GF) c v xs front dq d leaf kind context hd hdepth hp
          (by simpa only [List.append_nil] using hbytes)
    | @count leaf head payload tail argument perCount enc hm bound short =>
      have hpos : v.pos' + W64 (front.length + (1 + payload.length) : Nat) =
          v.pos' + W64 front.length + W64 (1 + payload.length) := by word
      simpa only [skipFailureAddr, SkipFailureError, hpos] using
        wp_Skip_context_count (GF := GF) c v xs front payload tail head argument perCount dq d leaf kind context enc hm bound short hd hdepth hp hbytes
    | @containerDepth head payload tail argument perCount enc hm =>
      have hpos : v.pos' + W64 (front.length + (1 + payload.length) : Nat) =
          v.pos' + W64 front.length + W64 (1 + payload.length) := by word
      simpa only [skipFailureAddr, SkipFailureError, hpos] using
        wp_Skip_context_container_depth (GF := GF) c v xs front payload tail head argument perCount dq d kind context enc hm hd hdepth hp hbytes
    | @tagEOF leaf head payload argument enc hm number =>
      wp_start_folded as ⟨#Hkind, Hc, Hs⟩
      isimp only [skipFailureAddr] at Hkind
      ihave %Hlen := ownSlice_len _ _ _ $$ Hs
      have hlen := congrArg List.length hbytes
      simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_append, List.length_cons] at hlen
      have heof : v.pos' + W64 (front.length + (1 + payload.length) : Nat) = v.buf'.len := by
        simp only [sint.nat, sint.Z] at Hlen hp hlen
        word
      isimp only [SkipFailureError, heof] at HΦ
      wp_apply wp_Skip_context_tag_eof c v xs front payload head argument dq d leaf kind context enc hm number hd hdepth hp hbytes
        $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
      · iframe Hkind Hc Hs
      wp_end
    | @tagDepth head next payload tail argument enc hm number nextTag =>
      have hpos : v.pos' + W64 (front.length + (1 + payload.length) : Nat) =
          v.pos' + W64 front.length + W64 (1 + payload.length) := by word
      simpa only [skipFailureAddr, SkipFailureError, hpos] using
        wp_Skip_context_tag_depth (GF := GF) c v xs front payload tail head next argument dq d kind context enc hm number nextTag hd hdepth hp hbytes

/-- The unified public rejection contract also bounds the exact diagnostic:
    its offset cannot wrap, and it is inside the original input or at EOF. -/
theorem wp_Skip_failure_bounded (c : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (d offset : Nat) (reason : SkipFailureReason) (kind : GoInterface)
    (failure : SkipFailure d wire offset reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       skipFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ SkipFailureError err (v.pos' + W64 offset) reason kind ∗
       ⌜sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hoff := skipFailure_offset d offset wire reason failure
  have Hnext := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_nil] using hbytes)
  have hpos : sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len := by constructor <;> word
  wp_apply wp_Skip_failure c v xs wire dq d offset reason kind failure hd hdepth hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_failure_bounded
