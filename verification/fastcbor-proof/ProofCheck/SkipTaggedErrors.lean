import ProofCheck.SkipTagPrefix

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

/-- Closed worker contract for any supported tag chain ending at a incomplete
    head. The leaf is discharged by its actual proved reader. -/
theorem wp_skip_tagged_truncated (c : Loc) (v : cursor.Cursor)
    (xs front tail : List w8) (bad : w8) (dq : DFrac) (depth : w64) (d leaf : Nat)
    (size : w64) (kind : GoInterface)
    (tags : SkipTagPrefix (bad :: tail) d front leaf) (hinfo : 24 ≤ uint.Z (bad &&& W8 31) ∧ uint.Z (bad &&& W8 31) ≤ 27)
    (hsize : W64 1 <<< W64 (uint.Z ((bad &&& W8 31) - W8 24)) = size)
    (htrunc : tail.length < sint.Z size)
    (hd : sint.Z depth = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front ++ (bad :: tail)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ {v with pos' := v.pos' + W64 front.length} ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind (v.pos' + W64 front.length) (go!"input ends inside the head") slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs front (bad :: tail) v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpos : 0 ≤ sint.Z (v.pos' + W64 front.length) ∧
      sint.Z (v.pos' + W64 front.length) ≤ sint.Z v.buf'.len := by constructor <;> word
  wp_apply wp_skip_tag_prefix_reject c v xs front (bad :: tail) dq depth
    (v.pos' + W64 front.length) d leaf iprop(globalAddr cursor.ErrTruncated ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind (v.pos' + W64 front.length) (go!"input ends inside the head") slice.nil))
    tags hd hdepth hp hbytes
    (wp_skip_head_truncated c {v with pos' := v.pos' + W64 front.length} (W64 leaf) xs tail bad size dq kind hinfo hsize htrunc hpos Hnext.2.2)
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Public Skip restores the entire original cursor while retaining a tagged
    incomplete head's inner offset and exact owned diagnostic. -/
theorem wp_Skip_tagged_truncated (c : Loc) (v : cursor.Cursor)
    (xs front tail : List w8) (bad : w8) (dq : DFrac) (d leaf : Nat)
    (size : w64) (kind : GoInterface)
    (tags : SkipTagPrefix (bad :: tail) d front leaf) (hinfo : 24 ≤ uint.Z (bad &&& W8 31) ∧ uint.Z (bad &&& W8 31) ≤ 27)
    (hsize : W64 1 <<< W64 (uint.Z ((bad &&& W8 31) - W8 24)) = size)
    (htrunc : tail.length < sint.Z size)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front ++ (bad :: tail)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind (v.pos' + W64 front.length) (go!"input ends inside the head") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_skip_tagged_truncated c v xs front tail bad dq v.depth' d leaf size kind tags hinfo hsize htrunc hd hdepth hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Closed worker contract for any supported tag chain ending at a atomic
    rejection. The leaf is discharged by its actual proved reader. -/
theorem wp_skip_tagged_atomic (c : Loc) (v : cursor.Cursor)
    (xs front payload tail : List w8) (bad : w8) (dq : DFrac) (depth : w64) (d leaf : Nat)
    (argument : w64) (reason : SkipAtomError) (kind : GoInterface)
    (tags : SkipTagPrefix ((bad :: payload) ++ tail) d front leaf) (enc : HeadArgument bad payload argument) (reject : SkipReject bad argument tail reason)
    (hd : sint.Z depth = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front ++ ((bad :: payload) ++ tail)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       skipAtomErrorAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ {v with pos' := v.pos' + W64 front.length + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind (v.pos' + W64 front.length + W64 (1 + payload.length)) (skipAtomErrorMessage reason) slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs front ((bad :: payload) ++ tail) v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpos : 0 ≤ sint.Z (v.pos' + W64 front.length) ∧
      sint.Z (v.pos' + W64 front.length) ≤ sint.Z v.buf'.len := by constructor <;> word
  wp_apply wp_skip_tag_prefix_reject c v xs front ((bad :: payload) ++ tail) dq depth
    (v.pos' + W64 front.length + W64 (1 + payload.length)) d leaf iprop(skipAtomErrorAddr (GF := GF) reason ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind (v.pos' + W64 front.length + W64 (1 + payload.length)) (skipAtomErrorMessage reason) slice.nil))
    tags hd hdepth hp hbytes
    (wp_skip_reject c {v with pos' := v.pos' + W64 front.length} (W64 leaf) xs payload tail bad argument dq reason kind enc reject hpos Hnext.2.2)
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Public Skip restores the entire original cursor while retaining a tagged
    atomic rejection's post-head offset and exact owned diagnostic. -/
theorem wp_Skip_tagged_atomic (c : Loc) (v : cursor.Cursor)
    (xs front payload tail : List w8) (bad : w8) (dq : DFrac) (d leaf : Nat)
    (argument : w64) (reason : SkipAtomError) (kind : GoInterface)
    (tags : SkipTagPrefix ((bad :: payload) ++ tail) d front leaf) (enc : HeadArgument bad payload argument) (reject : SkipReject bad argument tail reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front ++ ((bad :: payload) ++ tail)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       skipAtomErrorAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind (v.pos' + W64 front.length + W64 (1 + payload.length)) (skipAtomErrorMessage reason) slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_skip_tagged_atomic c v xs front payload tail bad dq v.depth' d leaf argument reason kind tags enc reject hd hdepth hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_tagged_truncated
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_tagged_truncated
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_tagged_atomic
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_tagged_atomic
