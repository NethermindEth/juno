import ProofCheck.SkipContextErrors

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

/-- Any non-unsigned major excludes a complete wide unsigned prefix. -/
theorem skip_major_nonWideUint (head : w8) (payload rem : List w8)
    (hm : head &&& W8 224 ≠ W8 0) : SkipNonWideUint ((head :: payload) ++ rem) := by
  intro hh
  have heq : head = W8 27 := by simpa using hh
  subst head; contradiction

/-- Count errors propagate through every enclosing supported context, retaining
    the end-of-head diagnostic while public Skip restores the original cursor. -/
theorem wp_Skip_context_count (c : Loc) (v : cursor.Cursor)
    (xs front payload rem : List w8) (head : w8) (count perCount : w64) (dq : DFrac) (d leaf : Nat) (kind : GoInterface)
    (context : SkipFailureContext ((head :: payload) ++ rem) d front leaf)
    (enc : HeadArgument head payload count)
    (hm : (head &&& W8 224 = W8 128 ∧ perCount = W64 1) ∨ (head &&& W8 224 = W8 160 ∧ perCount = W64 2))
    (hlimit : leaf < 32) (hshort : rem.length < uint.Z count * uint.Z perCount)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front ++ ((head :: payload) ++ rem)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind
         (v.pos' + W64 front.length + W64 (1 + payload.length)) (go!"count exceeds the input") slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs front ((head :: payload) ++ rem) v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpos : 0 ≤ sint.Z (v.pos' + W64 front.length) ∧
      sint.Z (v.pos' + W64 front.length) ≤ sint.Z v.buf'.len := by constructor <;> word
  have hn : head &&& W8 224 ≠ W8 0 := by
    rcases hm with ⟨hm, _⟩ | ⟨hm, _⟩ <;> rw [hm] <;> decide
  wp_apply wp_Skip_context_reject c v xs front ((head :: payload) ++ rem) dq
    (v.pos' + W64 front.length + W64 (1 + payload.length)) d leaf iprop(globalAddr cursor.ErrTruncated ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind (v.pos' + W64 front.length + W64 (1 + payload.length)) (go!"count exceeds the input") slice.nil))
    context (skip_major_nonWideUint head payload rem hn)
    (fun _ _ => skip_count_not_wide head payload rem count perCount enc hm hshort) hd hdepth hp hbytes
    (wp_skip_container_count c {v with pos' := v.pos' + W64 front.length} xs payload rem head count perCount (W64 leaf) dq kind
      enc hm (by constructor <;> word) hshort hpos Hnext.2.2) $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Contexts reaching an array/map at depth 32 preserve its owned depth error,
    including when its bytes have a complete wide-felt shape. -/
theorem wp_Skip_context_container_depth (c : Loc) (v : cursor.Cursor)
    (xs front payload rem : List w8) (head : w8) (count perCount : w64) (dq : DFrac) (d : Nat) (kind : GoInterface)
    (context : SkipFailureContext ((head :: payload) ++ rem) d front 32)
    (enc : HeadArgument head payload count)
    (hm : (head &&& W8 224 = W8 128 ∧ perCount = W64 1) ∨ (head &&& W8 224 = W8 160 ∧ perCount = W64 2))
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front ++ ((head :: payload) ++ rem)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrDepth ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind (v.pos' + W64 front.length + W64 (1 + payload.length))
         (go!"nested more than %d levels") args ∗ args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs front ((head :: payload) ++ rem) v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpos : 0 ≤ sint.Z (v.pos' + W64 front.length) ∧
      sint.Z (v.pos' + W64 front.length) ≤ sint.Z v.buf'.len := by constructor <;> word
  have hn : head &&& W8 224 ≠ W8 0 := by
    rcases hm with ⟨hm, _⟩ | ⟨hm, _⟩ <;> rw [hm] <;> decide
  wp_apply wp_Skip_context_reject c v xs front ((head :: payload) ++ rem) dq
    (v.pos' + W64 front.length + W64 (1 + payload.length)) d 32 iprop(globalAddr cursor.ErrDepth ↦□ kind)
    (fun err => iprop(∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind (v.pos' + W64 front.length + W64 (1 + payload.length))
         (go!"nested more than %d levels") args ∗ args ↦* [interface.mkOk go.int #(W64 32)]))
    context (skip_major_nonWideUint head payload rem hn) (by intro h; omega) hd hdepth hp hbytes
    (wp_skip_container_depth c {v with pos' := v.pos' + W64 front.length} xs payload rem head count perCount dq kind
      enc hm hpos Hnext.2.2) $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- A supported tag with missing content reports EOF through all enclosing
    containers/tags, with whole-input ownership and complete public rollback. -/
theorem wp_Skip_context_tag_eof (c : Loc) (v : cursor.Cursor)
    (xs front payload : List w8) (head : w8) (argument : w64) (dq : DFrac) (d leaf : Nat) (kind : GoInterface)
    (context : SkipFailureContext (head :: payload) d front leaf)
    (enc : HeadArgument head payload argument) (hm : head &&& W8 224 = W8 192) (hnumber : 3 < uint.Z argument)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front ++ (head :: payload)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind v.buf'.len (go!"input ends before the item") slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs front (head :: payload) v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpos : 0 ≤ sint.Z (v.pos' + W64 front.length) ∧
      sint.Z (v.pos' + W64 front.length) ≤ sint.Z v.buf'.len := by constructor <;> word
  have short : SkipNonWideUint (head :: payload) := by
    simpa only [List.append_nil] using skip_major_nonWideUint head payload [] (by rw [hm]; decide)
  have rejected : SkipWideRejected leaf (head :: payload) := by
    intro hd hl shape
    have heq : head = W8 132 := by simpa using shape.1
    subst head; contradiction
  wp_apply wp_Skip_context_reject c v xs front (head :: payload) dq v.buf'.len d leaf
    iprop(globalAddr cursor.ErrTruncated ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.buf'.len (go!"input ends before the item") slice.nil))
    context short rejected hd hdepth hp hbytes
    (wp_skip_tag_eof c {v with pos' := v.pos' + W64 front.length} xs payload head argument (W64 leaf) dq kind
      enc hm hnumber hpos Hnext.2.2) $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- A chained tag beyond the depth limit is rejected before its child is read,
    and all enclosing contexts preserve that diagnostic and restore the cursor. -/
theorem wp_Skip_context_tag_depth (c : Loc) (v : cursor.Cursor)
    (xs front payload tail : List w8) (head next : w8) (argument : w64) (dq : DFrac) (d : Nat) (kind : GoInterface)
    (context : SkipFailureContext ((head :: payload) ++ (next :: tail)) d front 32)
    (enc : HeadArgument head payload argument) (hm : head &&& W8 224 = W8 192)
    (hnumber : 3 < uint.Z argument) (ht : next &&& W8 224 = W8 192)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front ++ ((head :: payload) ++ (next :: tail))) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrDepth ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind (v.pos' + W64 front.length + W64 (1 + payload.length))
         (go!"nested more than %d levels") args ∗ args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs front ((head :: payload) ++ (next :: tail)) v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpos : 0 ≤ sint.Z (v.pos' + W64 front.length) ∧
      sint.Z (v.pos' + W64 front.length) ≤ sint.Z v.buf'.len := by constructor <;> word
  wp_apply wp_Skip_context_reject c v xs front ((head :: payload) ++ (next :: tail)) dq
    (v.pos' + W64 front.length + W64 (1 + payload.length)) d 32 iprop(globalAddr cursor.ErrDepth ↦□ kind)
    (fun err => iprop(∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind (v.pos' + W64 front.length + W64 (1 + payload.length))
         (go!"nested more than %d levels") args ∗ args ↦* [interface.mkOk go.int #(W64 32)]))
    context (skip_major_nonWideUint head payload (next :: tail) (by rw [hm]; decide)) (by intro h; omega)
    hd hdepth hp hbytes
    (wp_skip_tag_depth c {v with pos' := v.pos' + W64 front.length} xs payload tail [] head next argument dq kind
      enc hm hnumber ht hpos (by simpa only [List.append_nil] using Hnext.2.2))
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.skip_major_nonWideUint
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_context_count
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_context_container_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_context_tag_eof
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_context_tag_depth
