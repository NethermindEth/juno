import ProofCheck.IntegerKeySpec

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

def integerKeyValue (key : Option w64) : w64 := key.getD (W64 0)

theorem wp_readIntegerKey_valid (validator : Utf8ValidatorContract (GF := GF))
    (c : Loc) (v : cursor.Cursor) (xs item rest : List w8) (key : Option w64) (dq : DFrac)
    (valid : IntegerKeyValid item key)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (@! cborReadIntegerKey)) (Val #c))
    {{ RET (PairV (PairV #(integerKeyValue key) #key.isSome) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  cases valid with
  | unsigned enc major bound =>
    exact wp_readIntegerKey_unsigned c v xs _ rest _ _ dq enc major bound hp hbytes
  | unknown valid notUnsigned =>
    cases valid with
    | @integer head payload argument enc major bound =>
      have hn : head &&& W8 224 ≠ W8 0 := fun h => notUnsigned (congrArg some h)
      have negative : head &&& W8 224 = W8 32 := major.resolve_left hn
      have hsize : ((head :: payload).length : Int) = 1 + payload.length := by simp; omega
      simpa only [integerKeyValue, Option.getD, Option.isSome, hsize, Int.add_assoc] using
        wp_readIntegerKey_negative (GF := GF) c v xs payload rest head argument dq enc negative bound hp hbytes
    | @text head payload body argument enc major count utf8 =>
      have heq : argument = W64 body.length := by word
      subst argument
      have hsize : (((head :: payload) ++ body).length : Int) = 1 + payload.length + body.length := by simp; omega
      simpa only [integerKeyValue, Option.getD, Option.isSome, hsize, Int.add_assoc, List.append_assoc] using
        wp_readIntegerKey_text (GF := GF) validator c v xs payload body rest head dq enc major utf8 hp hbytes

def integerKeyOtherKind (k : BigIntErrorKinds) : KeyFailureReason → Loc
  | .head _ reason => bigIntHeadKind k reason
  | .incomplete | .eof | .textTruncated => k.truncated
  | .integerRange | .typeMismatch _ => k.typeError
  | .tag => k.unsupported
  | .invalidUTF8 => k.syntaxError

def IntegerKeyError (result : GoInterface) (pos : w64) (reason : IntegerKeyFailureReason)
    (k : BigIntErrorKinds) : IProp GF :=
  match reason with
  | .overflow => iprop(⌜result = bigIntCategory k.typeError⌝)
  | .unsigned reason => iprop(∃ err : Loc,
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      ExpectedFailureError err pos (W8 0) reason (bigIntCategory (bigIntExpectedKind k reason)))
  | .other reason => iprop(∃ err : Loc,
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      KeyFailureError err pos reason (bigIntCategory (integerKeyOtherKind k reason)))

theorem wp_readIntegerKey_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c : Loc) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (consumed : Nat)
    (reason : IntegerKeyFailureReason) (k : BigIntErrorKinds)
    (failure : IntegerKeyRejected wire consumed reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       BigIntErrorGlobals k ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (@! cborReadIntegerKey)) (Val #c))
    {{ (result : GoInterface), RET (PairV (PairV #(W64 0) #false) #result);
       c ↦ {v with pos' := v.pos' + W64 consumed} ∗ v.buf' ↦*{dq} xs ∗ IntegerKeyError result v.pos' reason k }} := by
  wp_start_folded as ⟨#Hglobals, Hc, Hs⟩
  isimp only [BigIntErrorGlobals] at Hglobals
  icases Hglobals with ⟨#Htruncated, #Hsyntax, #Htype, #Hunsupported, _⟩
  have hz (x : w64) : x + W64 0 = x := by word
  cases failure with
  | @unsigned head tail reason major failure =>
    ihave #Hkind : (expectedFailureAddr (GF := GF) reason ↦□ bigIntCategory (bigIntExpectedKind k reason) : IProp GF) $$ []
    · cases reason with
      | head _ reason => cases reason <;> isimp only [expectedFailureAddr, headFaultAddr, bigIntExpectedKind, bigIntHeadKind] <;> (iframe # <;> iframe)
      | incomplete | eof | typeMismatch _ | tag => isimp only [expectedFailureAddr, bigIntExpectedKind]; iframe # <;> iframe
    wp_apply wp_readIntegerKey_unsigned_failure c v xs tail head dq reason _ major failure hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    iapply HΦ
    isimp only [IntegerKeyError, Int.natCast_zero, hz]
    iframe Hc Hs
    iexists err
    iframe Herr
    ipureintro; rfl
  | @overflow head payload tail argument enc major bound =>
    wp_apply wp_readIntegerKey_overflow c v xs payload tail head argument dq (bigIntCategory k.typeError)
      enc major bound hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Htype Hc Hs
    iapply HΦ
    iframe Hc Hs
    isimp only [IntegerKeyError]
    ipureintro; trivial
  | @other wire reason major failure =>
    ihave #Hkind : (keyFailureAddr (GF := GF) reason ↦□ bigIntCategory (integerKeyOtherKind k reason) : IProp GF) $$ []
    · cases reason with
      | head _ reason => cases reason <;> isimp only [keyFailureAddr, headFaultAddr, integerKeyOtherKind, bigIntHeadKind] <;> (iframe # <;> iframe)
      | incomplete | eof | integerRange | tag | typeMismatch _ | textTruncated | invalidUTF8 =>
        isimp only [keyFailureAddr, integerKeyOtherKind]; iframe # <;> iframe
    cases wire with
    | nil =>
      cases failure
      ihave %Hlen := ownSlice_len _ _ _ $$ Hs
      have hlen := congrArg List.length hbytes
      simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_nil] at hlen
      have heof : v.pos' = v.buf'.len := by simp only [sint.nat] at Hlen hlen; word
      wp_apply wp_readIntegerKey_eof c v xs dq (bigIntCategory k.truncated) heof $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
      · iframe Htruncated Hc Hs
      iapply HΦ
      isimp only [IntegerKeyError, KeyFailureError, integerKeyOtherKind, Int.natCast_zero, hz]
      iframe Hc Hs
      iexists err
      iframe Herr
      ipureintro; rfl
    | cons head tail =>
      have hn : head &&& W8 224 ≠ W8 0 := fun h => major (congrArg some h)
      wp_apply wp_readIntegerKey_other_failure validator c v xs tail head dq reason _ hn failure hp hbytes
        $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
      · iframe Hkind Hc Hs
      iapply HΦ
      isimp only [IntegerKeyError, Int.natCast_zero, hz]
      iframe Hc Hs
      iexists err
      iframe Herr
      ipureintro; rfl

def IntegerKeyOutcome (c : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (key : w64) (unsigned : Bool) (result : GoInterface) (k : BigIntErrorKinds) : IProp GF := iprop(
  (∃ (item rest : List w8) (selected : Option w64),
    c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜key = integerKeyValue selected ∧ unsigned = selected.isSome ∧ result = interface.nil ∧
      wire = item ++ rest ∧ IntegerKeyValid item selected ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (consumed : Nat) (reason : IntegerKeyFailureReason),
    c ↦ {v with pos' := v.pos' + W64 consumed} ∗ IntegerKeyError result v.pos' reason k ∗
    ⌜key = W64 0 ∧ unsigned = false ∧ IntegerKeyRejected wire consumed reason ∧
      sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len⌝))

/-- All-input integer-key dispatch, conditional only on the existing UTF-8
    validator. Known unsigned keys, unknown negative/text keys and all error
    precedence/consumption cases follow the independent contract. -/
theorem wp_readIntegerKey_complete (validator : Utf8ValidatorContract (GF := GF))
    (c : Loc) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (@! cborReadIntegerKey)) (Val #c))
    {{ (key : w64) (unsigned : Bool) (result : GoInterface), RET (PairV (PairV #key #unsigned) #result);
       v.buf' ↦*{dq} xs ∗ ∃ k : BigIntErrorKinds, BigIntErrorGlobals k ∗ IntegerKeyOutcome c v xs wire key unsigned result k }} := by
  wp_start_folded as ⟨Hc, Hs⟩
  ihave Hglobals := cursor_initialized_bigInt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%k, #Hglobals⟩
  rcases integerKey_classify wire with ⟨item, rest, selected, split, valid⟩ | ⟨consumed, reason, failure⟩
  · wp_apply wp_readIntegerKey_valid validator c v xs item rest selected dq valid hp (hbytes.trans split)
      $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
    · iframe Hc Hs
    iapply HΦ
    iframe Hs
    iexists k
    iframe Hglobals
    unfold IntegerKeyOutcome
    ileft
    iexists item, rest, selected
    iframe Hc
    ipureintro; exact ⟨rfl, rfl, rfl, split, valid, Hnext⟩
  · ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have bound := integerKeyRejected_bounds wire consumed reason failure
    have Hnext := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1
      (by simpa only [List.append_nil] using hbytes)
    have hpos : sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len := by constructor <;> word
    wp_apply wp_readIntegerKey_rejected validator c v xs wire dq consumed reason k failure hp hbytes
      $$ [Hc Hs] as %result ⟨Hc, Hs, Herr⟩
    · iframe Hglobals Hc Hs
    iapply HΦ
    iframe Hs
    iexists k
    iframe Hglobals
    unfold IntegerKeyOutcome
    iright
    iexists consumed, reason
    iframe Hc Herr
    ipureintro; exact ⟨rfl, rfl, failure, hpos⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readIntegerKey_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readIntegerKey_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readIntegerKey_complete
