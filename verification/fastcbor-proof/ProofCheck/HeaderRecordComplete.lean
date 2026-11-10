import ProofCheck.HeaderMapComplete
import ProofCheck.HeaderMapRejected
import ProofCheck.HeaderOpenContract

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

/-- Every successful complete Header record specification is realized from
    the public API's zero destination, conditional on the validator contract. -/
theorem wp_readHeader_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst fields gasFields : Loc) (v : cursor.Cursor) (f gasF : cursor.Fields)
    (xs wire rem : List w8) (final : HeaderValue) (kind : GoInterface)
    (qf qo qn qgf qgo qgn dq : DFrac) (d : Nat)
    (valid : HeaderRecordValid d headerZeroValue wire final) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (bytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ globalAddr internal.cborschema.cborGasPriceFields ↦□ gasFields ∗ globalAddr cursor.ErrUnsupported ↦□ kind ∗
       dst ↦ (zero_val Header) ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames }}
      (App (App (Val (@! cborReadHeader)) (Val #c)) (Val #dst))
    {{ RET #interface.nil;
       HeaderOwned dst final ∗ c ↦ {v with pos' := v.pos' + W64 wire.length} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  iintro %Φ ⟨#Hpkg, #Hglobal, #HgasGlobal, #Hkind, Hdst, Hc, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs wire rem v.pos' v.buf'.len hp Hlen.1 bytes
  cases valid with
  | @nullish head nullish =>
    ihave Hchildren := headerChildren_zero (GF := GF)
    wp_apply wp_readHeader_nullish_owned c dst fields (zero_val Header) headerZeroValue v xs rem head dq nullish hp bytes
      $$ [Hdst Hchildren Hc Hs] with ⟨Hdst, Hchildren, Hc, Hs, %Hposition⟩
    · iframe Hglobal Hdst Hchildren Hc Hs
    isimp only [List.length_singleton, Int.natCast_one] at HΦ
    iapply HΦ
    iframe Hc Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
    isplitl [Hdst Hchildren]
    · unfold HeaderOwned
      iexists zero_val Header
      iframe Hdst Hchildren
    ipureintro; exact Hposition
  | @map head payload body argument count final enc major countEq depth pairs =>
    have argumentEq : argument = W64 count := by word
    subst argument
    wp_apply wp_readHeader_map_valid validator c dst fields gasFields v f gasF xs payload body rem head
      kind qf qo qn qgf qgo qgn dq d count final enc major hd depth pairs hp bytes
      $$ [Hdst Hc Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames]
      with ⟨Howned, Hc, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames, %Hposition⟩
    · iframe Hglobal HgasGlobal Hkind Hdst Hc Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
    have length : (((head :: payload) ++ body).length : Int) = 1 + payload.length + body.length := by simp; omega
    isimp only [length] at HΦ
    iapply HΦ
    iframe Howned Hc Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
    ipureintro
    simpa only [length, Int.add_assoc] using Hnext

/-- Every complete Header record rejection, including opening errors and
    all map/child failures, preserves the exact partial record and diagnostic. -/
theorem wp_readHeader_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst fields gasFields : Loc) (v : cursor.Cursor) (f gasF : cursor.Fields)
    (xs wire : List w8) (final : HeaderValue) (kinds : FeltErrorKinds)
    (qf qo qn qgf qgo qgn dq : DFrac) (d consumed offset extraDepth : Nat) (reason : HeaderRecordFailureReason)
    (failure : HeaderRecordRejected d headerZeroValue wire final consumed offset extraDepth reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (bytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ globalAddr internal.cborschema.cborGasPriceFields ↦□ gasFields ∗ FeltErrorGlobals kinds ∗
       dst ↦ (zero_val Header) ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames }}
      (App (App (Val (@! cborReadHeader)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       HeaderOwned dst final ∗ c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames ∗
       HeaderRecordFailureError err (v.pos' + W64 offset) reason (headerRecordFailureKind reason kinds) ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧ d + extraDepth ≤ 32⌝ }} := by
  iintro %Φ ⟨#Hpkg, #Hglobal, #HgasGlobal, Hglobals, Hdst, Hc, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hwhole := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_nil] using bytes)
  have bounds := headerRecordRejected_bounds d consumed offset extraDepth headerZeroValue final wire reason hdepth failure
  have hprogress : sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧ d + extraDepth ≤ 32 := by
    exact ⟨by word, by word, by word, by word, bounds.2.2⟩
  cases failure with
  | @opening wire reason failure =>
    ihave #Hkind := feltErrorGlobals_lookup kinds reason $$ Hglobals
    wp_apply wp_readHeader_open_failure c dst fields (zero_val Header) v xs wire dq d reason (feltFailureKind reason kinds)
      failure hd hp bytes $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hglobal Hkind Hdst Hc Hs
    ihave Hchildren := headerChildren_zero (GF := GF)
    have hz (x : w64) : x + W64 0 = x := by word
    isimp only [HeaderRecordFailureError, headerRecordFailureKind, Int.natCast_zero, hz] at HΦ
    iapply HΦ
    iframe Hc Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames Herr
    isplitl [Hdst Hchildren]
    · unfold HeaderOwned
      iexists zero_val Header
      iframe Hdst Hchildren
    ipureintro
    simpa only [Int.natCast_zero, hz] using hprogress
  | @map head payload body argument count consumed offset extraDepth final reason enc major countEq depth fit failure =>
    have argumentEq : argument = W64 count := by word
    subst argument
    wp_apply wp_readHeader_map_rejected validator c dst fields gasFields v f gasF xs payload body head
      kinds qf qo qn qgf qgo qgn dq d count consumed offset extraDepth final reason enc major hd depth fit failure hp bytes
      $$ [Hglobals Hdst Hc Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames]
      as %err ⟨Howned, Hc, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames, Herr, %Hpositions⟩
    · iframe Hglobal HgasGlobal Hglobals Hdst Hc Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
    have hdepthStep : v.depth' + W64 (1 + extraDepth : Nat) = v.depth' + W64 1 + W64 extraDepth := by word
    simp only [Int.natCast_add, Int.natCast_one] at hdepthStep
    isimp only [Int.natCast_add, Int.natCast_one, hdepthStep] at HΦ
    iapply HΦ
    iframe Howned Hc Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames Herr
    ipureintro
    simpa only [Int.natCast_add, Int.natCast_one] using hprogress

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readHeader_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readHeader_rejected
