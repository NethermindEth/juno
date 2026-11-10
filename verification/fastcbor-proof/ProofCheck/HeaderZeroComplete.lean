import ProofCheck.HeaderValidAPI
import ProofCheck.HeaderRejectedAPI
import ProofCheck.HeaderInitializedAPI
import ProofCheck.HeaderAPISpec

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

/-- Concrete First results meet the independent public specification, including
    exact suffix aliasing and owned canonical diagnostics after cleanup. -/
def HeaderFirstZeroOutcome (dst : Loc) (input remaining : GoSlice) (wire : List w8)
    (result : GoInterface) : IProp GF := iprop(
  ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
  ((∃ (item rest : List w8) (final : HeaderValue), HeaderOwned dst final ∗
    ⌜result = interface.nil ∧ remaining = slice.slice input w8 (W64 item.length) input.len ∧
      wire = item ++ rest ∧ HeaderFirstSpec headerZeroValue wire (.success final rest) ∧
      sint.Z remaining.len = rest.length ∧ subslice (sint.nat (W64 item.length)) (sint.nat input.len) wire = rest⌝) ∨
   (∃ (offset : Nat) (reason : HeaderRecordFailureReason) (err : Loc), HeaderOwned dst headerZeroValue ∗
    HeaderRecordFailureError err (W64 offset) reason (headerRecordFailureKind reason kinds) ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧ remaining = slice.nil ∧
      HeaderFirstSpec headerZeroValue wire (.failure headerZeroValue (.decode offset reason)) ∧
      sint.Z (W64 offset) = offset ∧ offset ≤ wire.length⌝)))

/-- All byte inputs through actual DecodeHeaderFirst from a zero destination,
    deriving metadata and error globals from proved relevant initialization.
    The validator remains the only explicit unproved helper contract. -/
theorem wp_decodeHeaderFirst_zero_complete (validator : Utf8ValidatorContract (GF := GF))
    (dst : Loc) (input : GoSlice) (wire : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #input)) (Val #dst))
    {{ (remaining : GoSlice) (result : GoInterface), RET (PairV #remaining #result);
       input ↦*{dq} wire ∗ HeaderFirstZeroOutcome dst input remaining wire result }} := by
  iintro %Φ ⟨#Hpkg, Hdst, Hs⟩ HΦ
  ihave #Hmeta := header_initialized_metadata (GF := GF) $$ []
  · solve_pkg_init
  isimp only [HeaderMetadata] at Hmeta
  icases Hmeta with ⟨%fields, %gasFields, #Hglobal, #HgasGlobal, #Htable, #HgasTable⟩
  isimp only [StaticFieldsShared] at Htable HgasTable
  icases Htable with ⟨%f, #Hfields, #Horder, #Hnames, %_⟩
  icases HgasTable with ⟨%gasF, #HgasFields, #HgasOrder, #HgasNames, %_⟩
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  ihave #Hkind : (globalAddr cursor.ErrUnsupported ↦□ kinds.unsupportedKind : IProp GF) $$ []
  · isimp only [FeltErrorGlobals, SkipErrorGlobals] at Hglobals
    icases Hglobals with ⟨⟨_, _, #Hkind, _⟩, _⟩
    iexact Hkind
  rcases headerRecord_classify 0 headerZeroValue wire (by decide) with
    ⟨item, rest, final, split, valid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
  · wp_apply wp_decodeHeaderFirst_valid validator dst fields gasFields input f gasF item final valid rest kinds.unsupportedKind
      DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard dq $$ [Hdst Hs]
      with ⟨Hdst, Hs, _, _, _, _, _, _, %Hrest⟩
    · isimp only [split] at Hs
      iframe Hglobal HgasGlobal Hkind Hfields Horder Hnames HgasFields HgasOrder HgasNames Hdst Hs
    isimp only [← split] at Hs
    iapply HΦ
    iframe Hs
    unfold HeaderFirstZeroOutcome
    iexists kinds
    iframe Hglobals
    ileft
    iexists item, rest, final
    iframe Hdst
    ipureintro
    refine ⟨rfl, rfl, split, ?_, Hrest.1, ?_⟩
    · rw [split]; exact .success valid
    · simpa only [← split] using Hrest.2
  · wp_apply wp_decodeHeaderFirst_rejected validator dst fields gasFields input f gasF wire retained
      consumed offset extraDepth reason failure kinds
      DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard dq $$ [Hdst Hs]
      as %err ⟨Hdst, Hs, _, _, _, _, _, _, Herr, %Hoffset⟩
    · iframe Hglobal HgasGlobal Hglobals Hfields Horder Hnames HgasFields HgasOrder HgasNames Hdst Hs
    iapply HΦ
    iframe Hs
    unfold HeaderFirstZeroOutcome
    iexists kinds
    iframe Hglobals
    iright
    iexists offset, reason, err
    iframe Hdst Herr
    ipureintro; exact ⟨rfl, rfl, .rejected failure, Hoffset⟩

/-- Full results meet the complete independent API specification. Trailing
    bytes are distinct from parser errors and both paths own the cleared Header. -/
def HeaderFullZeroOutcome (dst : Loc) (wire : List w8) (result : GoInterface) : IProp GF := iprop(
  ∃ (kinds : FeltErrorKinds) (extraKind : GoInterface), FeltErrorGlobals kinds ∗
  globalAddr cborerr.ErrExtraneousData ↦□ extraKind ∗
  ((∃ final : HeaderValue, HeaderOwned dst final ∗
    ⌜result = interface.nil ∧ HeaderFullSpec headerZeroValue wire (.success final)⌝) ∨
   (∃ (offset : Nat) (reason : HeaderRecordFailureReason) (err : Loc), HeaderOwned dst headerZeroValue ∗
    HeaderRecordFailureError err (W64 offset) reason (headerRecordFailureKind reason kinds) ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      HeaderFullSpec headerZeroValue wire (.failure headerZeroValue (.decode offset reason)) ∧
      sint.Z (W64 offset) = offset ∧ offset ≤ wire.length⌝) ∨
   (∃ (extra : Nat) (err : Loc), HeaderOwned dst headerZeroValue ∗
    err ↦ {(zero_val cborerr.DecodeError) with Kind' := extraKind, Extra' := W64 extra} ∗
    ⌜result = interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err ∧
      HeaderFullSpec headerZeroValue wire (.failure headerZeroValue (.trailing extra))⌝)))

/-- All byte inputs through actual DecodeHeader, with full independent-spec
    outcomes and canonical errors obtained from relevant initialized packages.
    The validator remains an explicit unresolved premise. -/
theorem wp_decodeHeader_zero_complete (validator : Utf8ValidatorContract (GF := GF))
    (dst : Loc) (input : GoSlice) (wire : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       input ↦*{dq} wire ∗ HeaderFullZeroOutcome dst wire result }} := by
  iintro %Φ ⟨#Hpkg, #HentryPkg, Hdst, Hs⟩ HΦ
  ihave #Hmeta := header_initialized_metadata (GF := GF) $$ []
  · solve_pkg_init
  isimp only [HeaderMetadata] at Hmeta
  icases Hmeta with ⟨%fields, %gasFields, #Hglobal, #HgasGlobal, #Htable, #HgasTable⟩
  isimp only [StaticFieldsShared] at Htable HgasTable
  icases Htable with ⟨%f, #Hfields, #Horder, #Hnames, %_⟩
  icases HgasTable with ⟨%gasF, #HgasFields, #HgasOrder, #HgasNames, %_⟩
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  ihave #Hkind : (globalAddr cursor.ErrUnsupported ↦□ kinds.unsupportedKind : IProp GF) $$ []
  · isimp only [FeltErrorGlobals, SkipErrorGlobals] at Hglobals
    icases Hglobals with ⟨⟨_, _, #Hkind, _⟩, _⟩
    iexact Hkind
  ihave Hobjects := entryErrors_initialized_objects (GF := GF) $$ []
  · solve_pkg_init
  isimp only [EntryErrorsInitialized, errors.newVar] at Hobjects
  icases Hobjects with ⟨_, ⟨%extra, #Hextra, _⟩⟩
  rcases headerRecord_classify 0 headerZeroValue wire (by decide) with
    ⟨item, rest, final, split, valid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
  · by_cases empty : rest = []
    · subst rest
      simp only [List.append_nil] at split
      subst wire
      wp_apply wp_decodeHeader_valid validator dst fields gasFields input f gasF item final valid kinds.unsupportedKind
        DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard dq $$ [Hdst Hs]
        with ⟨Hdst, Hs, _, _, _, _, _, _⟩
      · iframe Hglobal HgasGlobal Hkind Hfields Horder Hnames HgasFields HgasOrder HgasNames Hdst Hs
      iapply HΦ
      iframe Hs
      unfold HeaderFullZeroOutcome
      iexists kinds, _
      iframe Hglobals Hextra
      ileft
      iexists final
      iframe Hdst
      ipureintro
      refine ⟨rfl, .success ?_⟩
      simpa only [List.append_nil] using HeaderFirstSpec.success (rest := []) valid
    · wp_apply wp_decodeHeader_valid_trailing validator dst fields gasFields input f gasF item final valid rest kinds.unsupportedKind _
        DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard dq empty $$ [Hdst Hs]
        as %err ⟨Hdst, Hs, _, _, _, _, _, _, Herr⟩
      · isimp only [split] at Hs
        iframe Hglobal HgasGlobal Hkind Hextra Hfields Horder Hnames HgasFields HgasOrder HgasNames Hdst Hs
      isimp only [← split] at Hs
      iapply HΦ
      iframe Hs
      unfold HeaderFullZeroOutcome
      iexists kinds, _
      iframe Hglobals Hextra
      iright
      iright
      iexists rest.length, err
      iframe Hdst Herr
      ipureintro
      refine ⟨rfl, .trailing (value := final) ?_ empty⟩
      rw [split]; exact .success valid
  · wp_apply wp_decodeHeader_rejected validator dst fields gasFields input f gasF wire retained
      consumed offset extraDepth reason failure kinds
      DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard dq $$ [Hdst Hs]
      as %err ⟨Hdst, Hs, _, _, _, _, _, _, Herr, %Hoffset⟩
    · iframe Hglobal HgasGlobal Hglobals Hfields Horder Hnames HgasFields HgasOrder HgasNames Hdst Hs
    iapply HΦ
    iframe Hs
    unfold HeaderFullZeroOutcome
    iexists kinds, _
    iframe Hglobals Hextra
    iright
    ileft
    iexists offset, reason, err
    iframe Hdst Herr
    ipureintro; exact ⟨rfl, .firstError (.rejected failure), Hoffset⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_zero_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_zero_complete
