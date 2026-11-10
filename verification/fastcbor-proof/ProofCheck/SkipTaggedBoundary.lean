import ProofCheck.SkipTagBoundary

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

/-- An arbitrary supported tag chain ending without content reports the end of
    the input. Intermediate tags and the actual missing-child path are proved. -/
theorem wp_skip_tagged_eof (c : Loc) (v : cursor.Cursor)
    (xs front payload : List w8) (head : w8) (argument depth : w64)
    (dq : DFrac) (d leaf : Nat) (kind : GoInterface)
    (tags : SkipTagPrefix (head :: payload) d front leaf)
    (enc : HeadArgument head payload argument) (hm : head &&& W8 224 = W8 192)
    (hnumber : 3 < uint.Z argument) (hd : sint.Z depth = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front ++ (head :: payload)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ {v with pos' := v.buf'.len} ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.buf'.len (go!"input ends before the item") slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs front (head :: payload) v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpos : 0 ≤ sint.Z (v.pos' + W64 front.length) ∧
      sint.Z (v.pos' + W64 front.length) ≤ sint.Z v.buf'.len := by constructor <;> word
  wp_apply wp_skip_tag_prefix_reject c v xs front (head :: payload) dq depth v.buf'.len d leaf
    iprop(globalAddr cursor.ErrTruncated ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.buf'.len (go!"input ends before the item") slice.nil))
    tags hd hdepth hp hbytes
    (wp_skip_tag_eof c {v with pos' := v.pos' + W64 front.length} xs payload head argument (W64 leaf) dq kind
      enc hm hnumber hpos Hnext.2.2)
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Public Skip restores all cursor fields after missing tagged content. -/
theorem wp_Skip_tagged_eof (c : Loc) (v : cursor.Cursor)
    (xs front payload : List w8) (head : w8) (argument : w64)
    (dq : DFrac) (d leaf : Nat) (kind : GoInterface)
    (tags : SkipTagPrefix (head :: payload) d front leaf)
    (enc : HeadArgument head payload argument) (hm : head &&& W8 224 = W8 192)
    (hnumber : 3 < uint.Z argument) (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front ++ (head :: payload)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.buf'.len (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_skip_tagged_eof c v xs front payload head argument v.depth' dq d leaf kind
    tags enc hm hnumber hd hdepth hp hbytes $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- A tag prefix reaching the depth limit propagates the exact owned diagnostic
    when the next supported tag has another tagged child. -/
theorem wp_skip_tagged_depth (c : Loc) (v : cursor.Cursor)
    (xs front payload tail : List w8) (head next : w8) (argument depth : w64)
    (dq : DFrac) (d : Nat) (kind : GoInterface)
    (tags : SkipTagPrefix ((head :: payload) ++ (next :: tail)) d front 32)
    (enc : HeadArgument head payload argument) (hm : head &&& W8 224 = W8 192)
    (hnumber : 3 < uint.Z argument) (ht : next &&& W8 224 = W8 192)
    (hd : sint.Z depth = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front ++ ((head :: payload) ++ (next :: tail))) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrDepth ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ {v with pos' := v.pos' + W64 front.length + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       ∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind (v.pos' + W64 front.length + W64 (1 + payload.length))
         (go!"nested more than %d levels") args ∗ args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs front ((head :: payload) ++ (next :: tail)) v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpos : 0 ≤ sint.Z (v.pos' + W64 front.length) ∧
      sint.Z (v.pos' + W64 front.length) ≤ sint.Z v.buf'.len := by constructor <;> word
  wp_apply wp_skip_tag_prefix_reject c v xs front ((head :: payload) ++ (next :: tail)) dq depth
    (v.pos' + W64 front.length + W64 (1 + payload.length)) d 32 iprop(globalAddr cursor.ErrDepth ↦□ kind)
    (fun err => iprop(∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind (v.pos' + W64 front.length + W64 (1 + payload.length))
         (go!"nested more than %d levels") args ∗ args ↦* [interface.mkOk go.int #(W64 32)]))
    tags hd hdepth hp hbytes
    (wp_skip_tag_depth c {v with pos' := v.pos' + W64 front.length} xs payload tail [] head next argument dq kind
      enc hm hnumber ht hpos (by simpa only [List.append_nil] using Hnext.2.2))
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Public Skip restores its cursor even when a deeper tag chain exceeds depth. -/
theorem wp_Skip_tagged_depth (c : Loc) (v : cursor.Cursor)
    (xs front payload tail : List w8) (head next : w8) (argument : w64)
    (dq : DFrac) (d : Nat) (kind : GoInterface)
    (tags : SkipTagPrefix ((head :: payload) ++ (next :: tail)) d front 32)
    (enc : HeadArgument head payload argument) (hm : head &&& W8 224 = W8 192)
    (hnumber : 3 < uint.Z argument) (ht : next &&& W8 224 = W8 192)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front ++ ((head :: payload) ++ (next :: tail))) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrDepth ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       ∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind (v.pos' + W64 front.length + W64 (1 + payload.length))
         (go!"nested more than %d levels") args ∗ args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_skip_tagged_depth c v xs front payload tail head next argument v.depth' dq d kind
    tags enc hm hnumber ht hd hdepth hp hbytes $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_tagged_eof
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_tagged_eof
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_tagged_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_tagged_depth
