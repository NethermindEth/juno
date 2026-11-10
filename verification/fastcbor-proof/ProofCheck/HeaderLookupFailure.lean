import ProofCheck.HeaderMapDuplicate
import ProofCheck.HeaderMapKeyFailure

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

inductive HeaderLookupFailureReason where
  | duplicate (field : HeaderField)
  | key (reason : KeyFailureReason)

def headerLookupFailureKind (reason : HeaderLookupFailureReason) (kinds : FeltErrorKinds) : GoInterface :=
  match reason with
  | .duplicate _ => kinds.unsupportedKind
  | .key reason => keyFailureKind reason kinds

def HeaderLookupFailureError (err : Loc) (pos : w64) (reason : HeaderLookupFailureReason) (kind : GoInterface) : IProp GF :=
  match reason with
  | .key reason => KeyFailureError err pos reason kind
  | .duplicate field => iprop(∃ args : GoSlice,
      err ↦ cursor.DecodeError.mk kind pos (go!"field %s named twice") args ∗
      args ↦* [interface.mkOk go.string #(headerFieldWireName field)])

/-- Rejection while selecting the next Header field after an arbitrary prefix
    in HeaderEntry. Duplicate keys are consumed, while malformed keys leave the
    cursor at the key start. Both retain the prefix's complete application value.
    This is not an exhaustive Header error relation: field-value failures and
    UTF-8-dependent key failures are separate remaining obligations. -/
inductive HeaderLookupFailure : Nat → List w8 → HeaderValue → Nat → HeaderLookupFailureReason → Prop where
  | duplicate {d count : Nat} {head : w8} {payload tail : List w8} {field : HeaderField}
      (entries : List (HeaderEntry (d + 1))) (key : HeaderKeyWire)
      (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
      (depth : d < 32) (more : entries.length < count)
      (fit : 2 * count ≤ (entries.flatMap HeaderEntry.bytes ++ key.bytes ++ tail).length)
      (beforeDuplicate : HeaderEntriesBeforeDuplicate (W64 0) entries field) (choice : key.choice = some field) :
      HeaderLookupFailure d ((head :: payload) ++ entries.flatMap HeaderEntry.bytes ++ key.bytes ++ tail)
        (entries.foldl HeaderEntry.apply headerZeroValue)
        (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length + key.bytes.length) (.duplicate field)
  | key {d count : Nat} {head : w8} {payload tail : List w8} {reason : KeyFailureReason}
      (entries : List (HeaderEntry (d + 1)))
      (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
      (depth : d < 32) (more : entries.length < count)
      (fit : 2 * count ≤ (entries.flatMap HeaderEntry.bytes ++ tail).length)
      (fresh : HeaderEntriesFresh (W64 0) entries) (failure : KeyFailure tail reason) :
      HeaderLookupFailure d ((head :: payload) ++ entries.flatMap HeaderEntry.bytes ++ tail)
        (entries.foldl HeaderEntry.apply headerZeroValue)
        (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) (.key reason)

theorem headerLookupFailure_bounds (d offset : Nat) (wire : List w8) (value : HeaderValue)
    (reason : HeaderLookupFailureReason) (failure : HeaderLookupFailure d wire value offset reason) :
    offset ≤ wire.length ∧ d + 1 ≤ 32 := by
  cases failure <;> simp only [List.length_append, List.length_cons] <;> omega

/-- Closed composition through the actual Header reader. Prefix ownership and
    all diagnostic allocations survive until the public wrapper performs cleanup;
    the retained cursor, diagnostic and depth are bounded without word wrap. -/
theorem wp_readHeader_lookup_failure (c dst fields gasFields : Loc) (value : HeaderValue)
    (v : cursor.Cursor) (f gasF : cursor.Fields) (xs wire : List w8)
    (kinds : FeltErrorKinds) (qf qo qn qgf qgo qgn dq : DFrac) (d offset : Nat)
    (reason : HeaderLookupFailureReason) (failure : HeaderLookupFailure d wire value offset reason)
    (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cborHeaderFields ↦□ fields ∗ globalAddr cborGasPriceFields ↦□ gasFields ∗ FeltErrorGlobals kinds ∗
       dst ↦ (zero_val Header) ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames }}
      (App (App (Val (@! cborReadHeader)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       HeaderOwned dst value ∗ c ↦ {v with pos' := v.pos' + W64 offset, depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames ∗
       HeaderLookupFailureError err (v.pos' + W64 offset) reason (headerLookupFailureKind reason kinds) ∗
       ⌜sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧ d + 1 ≤ 32⌝ }} := by
  wp_start_folded as ⟨#Hglobal, #HgasGlobal, Hglobals, Hdst, Hc, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames⟩
  isimp only [FeltErrorGlobals, SkipErrorGlobals] at Hglobals
  icases Hglobals with ⟨⟨#Hsyntax, #Htruncated, #Hunsupported, #Hdepth⟩, #Htype⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hwhole := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_nil] using hbytes)
  have Hbounds := headerLookupFailure_bounds d offset wire value reason failure
  have hprogress : sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧ d + 1 ≤ 32 := by
    exact ⟨by word, by word, Hbounds.2⟩
  cases failure with
  | @duplicate count head payload tail field entries key enc major depth more fit beforeDuplicate choice =>
    wp_apply wp_readHeader_map_duplicate c dst fields gasFields v f gasF xs payload tail head kinds.unsupportedKind
      qf qo qn qgf qgo qgn dq d count entries key field enc major hd depth more fit beforeDuplicate choice hp hbytes
      $$ [Hdst Hc Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames]
      as %err ⟨%args, Hdst, Hc, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames, Herr, Hargs, %Hpositions⟩
    · iframe Hglobal HgasGlobal Hunsupported Hdst Hc Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
    have ho : v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length + key.bytes.length : Nat) =
        v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) + W64 key.bytes.length := by word
    simp only [Int.natCast_add, Int.natCast_one] at ho
    isimp only [HeaderLookupFailureError, headerLookupFailureKind, Int.natCast_add, Int.natCast_one, ho] at HΦ
    iapply HΦ
    iframe Hdst Hc Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
    isplitl [Herr Hargs]
    · iexists args
      iframe Herr Hargs
    ipureintro
    simpa only [Int.natCast_add, Int.natCast_one, ho] using hprogress
  | @key count head payload tail reason entries enc major depth more fit fresh failure =>
    ihave #Hkind : (keyFailureAddr (GF := GF) reason ↦□ keyFailureKind reason kinds : IProp GF) $$ []
    · iapply keyErrorGlobals_lookup kinds reason
      unfold FeltErrorGlobals SkipErrorGlobals
      iframe Hsyntax Htruncated Hunsupported Hdepth Htype
    wp_apply wp_readHeader_map_key_failure c dst fields gasFields v f gasF xs payload tail head kinds.unsupportedKind
      (keyFailureKind reason kinds) qf qo qn qgf qgo qgn dq d count entries reason enc major hd depth more fit fresh failure hp hbytes
      $$ [Hdst Hc Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames]
      as %err ⟨Hdst, Hc, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames, Herr, %Hpositions⟩
    · iframe Hglobal HgasGlobal Hunsupported Hkind Hdst Hc Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
    isimp only [HeaderLookupFailureError, headerLookupFailureKind, Int.natCast_add, Int.natCast_one] at HΦ
    iapply HΦ
    iframe Hdst Hc Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames Herr
    ipureintro
    simpa only [Int.natCast_add, Int.natCast_one] using hprogress

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerLookupFailure_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readHeader_lookup_failure
