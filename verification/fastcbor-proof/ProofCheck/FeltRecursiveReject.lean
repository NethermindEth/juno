import ProofCheck.SkipHeadFailureSound

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

/-- A malformed head arbitrarily nested inside ignored felt contents retains
    its inner diagnostic while restoring the whole felt cursor and old pointer. -/
theorem wp_readFelt_extras_headFailure (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload wire : List w8) (head bad : w8) (count : w64)
    (items : List UnsignedWire) (extras : List (List w8)) (dq : DFrac)
    (reason : HeadError) (kind : GoInterface)
    (d offset : Nat) (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hcount : items.length = 4) (hmore : 4 + extras.length < uint.Z count)
    (valid : ∀ item ∈ extras, SkipValid (d + 1) item) (failure : SkipHeadFailure bad reason (d + 1) wire offset)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ extras.flatten ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       headFaultAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       HeadFaultError err (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length) + W64 offset) bad reason kind }} := by
  wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  let pos := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length)
  have Hnext := input_consumption xs ((head :: payload) ++ unsignedBytes items ++ extras.flatten)
    wire v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpnext : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    dsimp only [pos]; constructor <;> word
  have hsuffix : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = wire := by
    simpa only [pos, List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm, Int.add_left_comm, Int.add_assoc] using Hnext.2.2
  have hbad :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         headFaultAddr (GF := GF) reason ↦□ kind ∗ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
      {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
         c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
         HeadFaultError err (pos + W64 offset) bad reason kind }} := by
    wp_start_folded as ⟨#Hkind, Hc, Hs⟩
    wp_apply wp_Skip_headFailure c {v with pos' := pos, depth' := v.depth' + W64 1} xs wire bad reason kind dq
      (d + 1) offset failure (by word) (by omega) hpnext hsuffix $$ [Hc Hs] as %err ⟨Hc, Hs, Herr, %Hoff⟩
    · iframe Hkind Hc Hs
    wp_end
  wp_apply wp_readFelt_extras_reject c dst old v xs payload wire head count items extras dq
    iprop(headFaultAddr (GF := GF) reason ↦□ kind)
    (fun err => HeadFaultError err (pos + W64 offset) bad reason kind)
    d hd hdepth enc hm hcount hmore valid hp hbytes
    hbad
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_extras_headFailure
