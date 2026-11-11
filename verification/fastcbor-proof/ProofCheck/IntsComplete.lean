import ProofCheck.IntsFailureSpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

def IntsFailureError (result : GoInterface) (pos : w64) (reason : IntsFailureReason)
    (k : BigIntErrorKinds) (depthKind : Loc) : IProp GF :=
  match reason with
  | .outer reason => iprop(∃ err : Loc,
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      ExpectedFailureError err pos (W8 128) reason (bigIntCategory (bigIntExpectedKind k reason)))
  | .count => iprop(∃ err : Loc,
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      err ↦ cursor.DecodeError.mk (bigIntCategory k.truncated) pos (go!"count exceeds the input") slice.nil)
  | .depth => iprop(∃ (err : Loc) (args : GoSlice),
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      err ↦ cursor.DecodeError.mk (bigIntCategory depthKind) pos (go!"nested more than %d levels") args ∗
      args ↦* [interface.mkOk go.int #(W64 32)])
  | .element reason => IntElementFailureError result pos reason k

/-- Every specified array rejection follows the actual implementation, preserving
    the destination and input while retaining exact cursor/depth and error ownership. -/
theorem wp_readInts_failure (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d consumed offset : Nat) (entered : Bool)
    (reason : IntsFailureReason) (k : BigIntErrorKinds) (depthKind : Loc)
    (failure : IntsRejected d wire consumed offset entered reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       BigIntErrorGlobals k ∗ globalAddr cursor.ErrDepth ↦□ bigIntCategory depthKind ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadInts)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       dst ↦ old ∗ c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 (if entered then 1 else 0 : Nat)} ∗
       v.buf' ↦*{dq} xs ∗ IntsFailureError result (v.pos' + W64 offset) reason k depthKind }} := by
  wp_start_folded as ⟨#Hglobals, #Hdepth, Hdst, Hc, Hs⟩
  isimp only [BigIntErrorGlobals] at Hglobals
  icases Hglobals with ⟨#Htruncated, #Hsyntax, #Htype, #Hunsupported, %Hne⟩
  have hz (x : w64) : x + W64 0 = x := by word
  cases failure with
  | @outer d wire reason hn hu failure =>
    ihave #Hkind : (expectedFailureAddr (GF := GF) reason ↦□ bigIntCategory (bigIntExpectedKind k reason) : IProp GF) $$ []
    · cases reason with
      | head _ reason => cases reason <;> isimp only [expectedFailureAddr, headFaultAddr, bigIntExpectedKind, bigIntHeadKind] <;> (iframe # <;> iframe)
      | incomplete | eof | typeMismatch _ | tag =>
        isimp only [expectedFailureAddr, bigIntExpectedKind]; iframe # <;> iframe
    wp_apply wp_readInts_outer_failure c dst old v xs _ dq _ _ hn hu failure hp hbytes
      $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    iapply HΦ
    isimp only [IntsFailureError, Bool.false_eq_true, ite_false, Int.natCast_zero, hz]
    iframe Hdst Hc Hs
    iexists err
    iframe Herr
    ipureintro; rfl
  | count enc major short =>
    wp_apply wp_readInts_count_failure c dst old v xs _ _ _ _ dq (bigIntCategory k.truncated)
      enc major short hp hbytes $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Htruncated Hdst Hc Hs
    iapply HΦ
    isimp only [IntsFailureError, Bool.false_eq_true, ite_false, Int.natCast_zero, hz]
    iframe Hdst Hc Hs
    iexists err
    iframe Herr
    ipureintro; rfl
  | depth enc major fit =>
    wp_apply wp_readInts_depth_failure c dst old v xs _ _ _ _ dq (bigIntCategory depthKind)
      enc major fit (by omega) hp hbytes $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, %args, Herr, Hargs⟩
    · iframe Hdepth Hdst Hc Hs
    iapply HΦ
    isimp only [IntsFailureError, Bool.false_eq_true, ite_false, Int.natCast_zero, hz]
    iframe Hdst Hc Hs
    iexists err, args
    iframe Herr Hargs
    ipureintro; rfl
  | @element d count consumed head payload wire items reason enc major depth more fit failure =>
    have hcpos : v.pos' + W64 (1 + payload.length + (items.flatMap IntElementWire.bytes).length + consumed : Nat) =
        v.pos' + W64 (1 + payload.length + (items.flatMap IntElementWire.bytes).length) + W64 consumed := by word
    simp only [Int.natCast_add, Int.natCast_one] at hcpos
    wp_apply wp_readInts_prefix_failure c dst old v xs payload wire head dq d count consumed items reason k
      enc major hd depth more fit failure hp hbytes $$ [Hdst Hc Hs] as %result ⟨Hdst, Hc, Hs, Herr⟩
    · isimp only [BigIntErrorGlobals]
      iframe # <;> iframe
      ipureintro; exact Hne
    iapply HΦ
    isimp only [IntsFailureError, ite_true, Int.natCast_add, Int.natCast_one, hcpos]
    iframe Hdst Hc Hs Herr

def IntsOutcome (c dst : Loc) (old : GoSlice) (v : cursor.Cursor) (xs wire : List w8)
    (d : Nat) (k : BigIntErrorKinds) (depthKind : Loc) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (value : Option (List Int)) (out : GoSlice),
    dst ↦ out ∗ IntsResult out value ∗ c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ IntsValid d item value ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (consumed offset : Nat) (entered : Bool) (reason : IntsFailureReason), dst ↦ old ∗
    c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 (if entered then 1 else 0 : Nat)} ∗
    IntsFailureError result (v.pos' + W64 offset) reason k depthKind ∗
    ⌜IntsRejected d wire consumed offset entered reason ∧
      sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z (v.pos' + W64 consumed) ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
      d + (if entered then 1 else 0 : Nat) ≤ 32⌝))

/-- Complete production signed-array decoding for every byte input at supported
    depth. Actual package initialization supplies error categories; every helper
    contract is proved. The source input remains fractionally owned and unchanged. -/
theorem wp_readInts_complete (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadInts)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ (k : BigIntErrorKinds) (depthKind : Loc),
       BigIntErrorGlobals k ∗ globalAddr cursor.ErrDepth ↦□ bigIntCategory depthKind ∗
       IntsOutcome c dst old v xs wire d k depthKind result }} := by
  wp_start_folded as ⟨Hdst, Hc, Hs⟩
  ihave Hglobals := cursor_initialized_bigInt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%k, #Hglobals⟩
  ihave Herrors := cursor_initialized_errors (GF := GF) $$ []
  · solve_pkg_init
  isimp only [CursorInitialized, CursorErrorGlobal] at Herrors
  icases Herrors with ⟨_, _, _, ⟨%depthKind, #Hdepth, _⟩, _⟩
  rcases ints_classify d wire hdepth with ⟨item, rest, value, split, valid⟩ | ⟨consumed, offset, entered, reason, failure⟩
  · wp_apply wp_readInts_valid c dst old v xs item rest value dq d hd hdepth valid hp (hbytes.trans split)
      $$ [Hdst Hc Hs] as %out ⟨Hdst, Hvalue, Hc, Hs, %Hnext⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    iexists k, depthKind
    iframe Hglobals Hdepth
    unfold IntsOutcome
    ileft
    iexists item, rest, value, out
    iframe Hdst Hvalue Hc
    ipureintro; exact ⟨rfl, split, valid, Hnext⟩
  · ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have hoff := intsRejected_bounds d consumed offset wire entered reason failure
    have hdep := intsRejected_depth d consumed offset wire entered reason hdepth failure
    have Hnext := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1
      (by simpa only [List.append_nil] using hbytes)
    have hpos : sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
        sint.Z (v.pos' + W64 offset) ≤ sint.Z (v.pos' + W64 consumed) ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len := by repeat' constructor <;> word
    wp_apply wp_readInts_failure c dst old v xs wire dq d consumed offset entered reason k depthKind
      failure hd hdepth hp hbytes $$ [Hdst Hc Hs] as %result ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hglobals Hdepth Hdst Hc Hs
    iapply HΦ
    iframe Hs
    iexists k, depthKind
    iframe Hglobals Hdepth
    unfold IntsOutcome
    iright
    iexists consumed, offset, entered, reason
    iframe Hdst Hc Herr
    ipureintro; exact ⟨failure, hpos.1, hpos.2.1, hpos.2.2.1, hpos.2.2.2, hdep⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readInts_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readInts_complete
