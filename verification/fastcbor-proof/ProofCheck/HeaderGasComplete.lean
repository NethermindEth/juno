import ProofCheck.HeaderGasFreshComplete

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

/-- All successful nullable price values through the actual Header field call,
    preserving the other fields and retaining shared allocation ownership. -/
theorem wp_headerGas_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst shared base fields : Loc) (h : Header) (old : HeaderValue) (second : Bool) (seen : w64)
    (v : cursor.Cursor) (f : cursor.Fields) (xs wire rem : List w8) (final : Option GasPriceValue)
    (kind : GoInterface) (qf qo qn dq : DFrac) (d : Nat)
    (fresh : HeaderKeyFresh seen (some (headerGasKey second)))
    (valid : GasPointerValid d wire final) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (bytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ globalAddr cursor.ErrUnsupported ↦□ kind ∗
       dst ↦ h ∗ HeaderBody h old ∗ shared ↦ base ∗ HeaderGasReady base h old seen ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c))
        (Val #(structFieldRef Header (headerGasName second) dst))) (Val #shared)) (Val #(headerGasSlot second)))
    {{ (out : Header) (outBase : Loc), RET #interface.nil;
       dst ↦ out ∗ HeaderBody out (headerSetGasValue second old final) ∗ shared ↦ outBase ∗
       HeaderGasReady outBase out (headerSetGasValue second old final) (headerKeySeen seen (some (headerGasKey second))) ∗
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  iintro %Φ ⟨#Hpkg, #Hglobal, #Hkind, Hh, Hbody, Hshared, Hready, Hc, Hs, Hfields, Horder, Hnames⟩ HΦ
  cases valid with
  | nullish hn =>
    wp_apply wp_headerGas_null_ready c dst shared base h old seen second v xs rem _ dq hn hp bytes
      $$ [Hh Hbody Hshared Hready Hc Hs] with ⟨Hh, Hbody, Hshared, Hready, Hc, Hs, %Hnext⟩
    · iframe Hh Hbody Hshared Hready Hc Hs
    isimp only [List.length_singleton, Int.natCast_one] at HΦ
    iapply HΦ
    iframe Hh Hbody Hshared Hready Hc Hs Hfields Horder Hnames
    ipureintro; simpa only [Int.add_assoc] using Hnext
  | @map head payload body argument count final enc major countEq depth valid =>
    have argumentEq : argument = W64 count := by word
    subst argument
    wp_apply wp_headerGas_fresh_map_valid validator c dst shared base fields h old second seen v f xs payload body rem head
      kind qf qo qn dq d count final fresh enc major hd depth valid hp bytes
      $$ [Hh Hbody Hshared Hready Hc Hs Hfields Horder Hnames]
      as %outBase ⟨Hh, Hbody, Hshared, Hready, Hc, Hs, Hfields, Horder, Hnames, %Hnext⟩
    · iframe Hglobal Hkind Hh Hbody Hshared Hready Hc Hs Hfields Horder Hnames
    have length : (((head :: payload) ++ body).length : Int) = 1 + payload.length + body.length := by simp; omega
    isimp only [length] at HΦ
    iapply HΦ
    iframe Hh Hbody Hshared Hready Hc Hs Hfields Horder Hnames
    ipureintro; simpa only [Int.add_assoc] using Hnext

/-- A price field either publishes the specified nullable value or preserves
    the entire public Header while retaining ownership of private partial data. -/
def HeaderGasOutcome (c dst shared : Loc) (h : Header) (old : HeaderValue) (second : Bool) (seen : w64)
    (v : cursor.Cursor) (xs wire : List w8) (d : Nat) (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (final : Option GasPriceValue) (out : Header) (outBase : Loc),
    dst ↦ out ∗ HeaderBody out (headerSetGasValue second old final) ∗ shared ↦ outBase ∗
    HeaderGasReady outBase out (headerSetGasValue second old final) (headerKeySeen seen (some (headerGasKey second))) ∗
    c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ GasPointerValid d item final ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (final : GasPriceValue) (consumed offset : Nat) (entered : Bool) (reason : GasPriceFailureReason) (err outBase : Loc),
    dst ↦ h ∗ HeaderBody h old ∗ shared ↦ outBase ∗ HeaderGasReady outBase h old (headerKeySeen seen (some (headerGasKey second))) ∗
    c ↦ {v with pos' := v.pos' + W64 consumed, depth' := if entered then v.depth' + W64 1 else v.depth'} ∗
    GasPriceFailureError err (v.pos' + W64 offset) reason (gasPriceFailureKind reason kinds) ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      GasRecordRejected d ⟨none, none⟩ wire final consumed offset entered reason ∧
      sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧ d + (if entered then 1 else 0) ≤ 32⌝))

/-- Arbitrary-input composition for either fresh Header GasPrice field. Only
    the production UTF-8 validator contract remains as an unresolved premise. -/
theorem wp_headerGas_complete (validator : Utf8ValidatorContract (GF := GF))
    (c dst shared base fields : Loc) (h : Header) (old : HeaderValue) (second : Bool) (seen : w64)
    (v : cursor.Cursor) (f : cursor.Fields) (xs wire : List w8)
    (kinds : FeltErrorKinds) (qf qo qn dq : DFrac) (d : Nat)
    (fresh : HeaderKeyFresh seen (some (headerGasKey second)))
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (bytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ FeltErrorGlobals kinds ∗
       dst ↦ h ∗ HeaderBody h old ∗ shared ↦ base ∗ HeaderGasReady base h old seen ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c))
        (Val #(structFieldRef Header (headerGasName second) dst))) (Val #shared)) (Val #(headerGasSlot second)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗
       HeaderGasOutcome c dst shared h old second seen v xs wire d kinds result }} := by
  iintro %Φ ⟨#Hpkg, #Hglobal, Hglobals, Hh, Hbody, Hshared, Hready, Hc, Hs, Hfields, Horder, Hnames⟩ HΦ
  rcases gasPointer_classify d wire hdepth with
    ⟨item, rest, final, split, valid⟩ | ⟨final, consumed, offset, entered, reason, failure⟩
  · isimp only [FeltErrorGlobals, SkipErrorGlobals] at Hglobals
    icases Hglobals with ⟨⟨_, _, #Hkind, _⟩, _⟩
    wp_apply wp_headerGas_valid validator c dst shared base fields h old second seen v f xs item rest final
      kinds.unsupportedKind qf qo qn dq d fresh valid hd hp (bytes.trans split)
      $$ [Hh Hbody Hshared Hready Hc Hs Hfields Horder Hnames]
      as %out %outBase ⟨Hh, Hbody, Hshared, Hready, Hc, Hs, Hfields, Horder, Hnames, %Hnext⟩
    · iframe Hglobal Hkind Hh Hbody Hshared Hready Hc Hs Hfields Horder Hnames
    iapply HΦ
    iframe Hs Hfields Horder Hnames
    unfold HeaderGasOutcome
    ileft
    iexists item, rest, final, out, outBase
    iframe Hh Hbody Hshared Hready Hc
    ipureintro; exact ⟨rfl, split, valid, Hnext⟩
  · wp_apply wp_headerGas_fresh_rejected validator c dst shared base fields h old second final seen v f xs wire
      kinds qf qo qn dq d consumed offset entered reason fresh failure hd hdepth hp bytes
      $$ [Hglobals Hh Hbody Hshared Hready Hc Hs Hfields Horder Hnames]
      as %err %outBase ⟨Hh, Hbody, Hshared, Hready, Hc, Hs, Hfields, Horder, Hnames, Herr, %Hpositions⟩
    · iframe Hglobal Hglobals Hh Hbody Hshared Hready Hc Hs Hfields Horder Hnames
    iapply HΦ
    iframe Hs Hfields Horder Hnames
    unfold HeaderGasOutcome
    iright
    iexists final, consumed, offset, entered, reason, err, outBase
    iframe Hh Hbody Hshared Hready Hc Herr
    ipureintro; exact ⟨rfl, failure, Hpositions⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerGas_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerGas_complete
