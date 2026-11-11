import ProofCheck.IndexesRejectedAPI
import ProofCheck.IndexesAPISpec
import ProofCheck.EntryErrorsInitialize

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

/-- Returned errors refer to actual initialized categories, with owned diagnostics
    when allocated and the precise bare sentinel for unsigned overflow. -/
def IndexesAPIErrorOwned (result : GoInterface) (error : IndexesAPIError) : IProp GF :=
  match error with
  | .nonzero => iprop(∃ (kind : GoInterface) (err : Loc), globalAddr cborerr.ErrUnsupported ↦□ kind ∗
      err ↦ {(zero_val cborerr.DecodeError) with Kind' := kind, Detail' := go!"the destination is not the zero value"} ∗
      ⌜result = interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err⌝)
  | .decode offset reason => iprop(∃ (k : BigIntErrorKinds) (depthKind : Loc),
      BigIntErrorGlobals k ∗ globalAddr cursor.ErrDepth ↦□ bigIntCategory depthKind ∗
      IndexesRecordError result (W64 offset) reason k depthKind ∗ ⌜sint.Z (W64 offset) = offset⌝)
  | .trailing extra => iprop(∃ (kind : GoInterface) (err : Loc), globalAddr cborerr.ErrExtraneousData ↦□ kind ∗
      err ↦ {(zero_val cborerr.DecodeError) with Kind' := kind, Extra' := W64 extra} ∗
      ⌜result = interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err⌝)

def IndexesFirstResultOwned (dst : Loc) (input remaining : GoSlice) (wire : List w8)
    (result : GoInterface) (outcome : IndexesFirstResult) : IProp GF :=
  match outcome with
  | .success final rest => iprop(IndexesOwned dst final ∗ ∃ item : List w8,
      ⌜result = interface.nil ∧ wire = item ++ rest ∧ remaining = slice.slice input w8 (W64 item.length) input.len ∧
        sint.Z remaining.len = rest.length ∧ subslice (sint.nat (W64 item.length)) (sint.nat input.len) wire = rest⌝)
  | .failure final error => iprop(IndexesOwned dst final ∗ IndexesAPIErrorOwned result error ∗ ⌜remaining = slice.nil⌝)

def IndexesFullResultOwned (dst : Loc) (result : GoInterface) (outcome : IndexesDecodeResult) : IProp GF :=
  match outcome with
  | .success final => iprop(IndexesOwned dst final ∗ ⌜result = interface.nil⌝)
  | .failure final error => iprop(IndexesOwned dst final ∗ IndexesAPIErrorOwned result error)

/-- Actual First decoding for every byte input and owned initial record. All
    helper/error initialization is derived; UTF-8 remains the explicit premise. -/
theorem wp_decodeIndexesFirst_complete (validator : Utf8ValidatorContract (GF := GF))
    (dst : Loc) (old : IndexesValue) (input : GoSlice) (wire : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       IndexesOwned dst old ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeBlockTransactionsIndexesFirst)) (Val #input)) (Val #dst))
    {{ (remaining : GoSlice) (result : GoInterface), RET (PairV #remaining #result);
       input ↦*{dq} wire ∗ ∃ outcome : IndexesFirstResult,
       ⌜IndexesFirstSpec old wire outcome⌝ ∗ IndexesFirstResultOwned dst input remaining wire result outcome }} := by
  iintro %Φ ⟨#Hpkg, #HentryPkg, Howned, Hs⟩ HΦ
  isimp only [IndexesOwned] at Howned
  icases Howned with ⟨%h, Hdst, Hchildren⟩
  ihave %Hzero := indexesChildren_zero_iff h old $$ Hchildren
  by_cases zero : old = indexesZeroValue
  · subst old
    have hz := Hzero.mpr rfl
    subst h
    rcases indexesRecord_classify 0 indexesZeroValue wire (by decide) with
      ⟨item, rest, final, split, valid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
    · subst wire
      wp_apply wp_decodeIndexesFirst_valid validator dst input item rest final dq valid $$ [Hdst Hs] with ⟨Hdst, Hs, %Hrest⟩
      · iframe Hdst Hs
      iapply HΦ
      iframe Hs
      iexists IndexesFirstResult.success final rest
      isplitl []
      · ipureintro; exact .success valid
      isimp only [IndexesFirstResultOwned]
      iframe Hdst
      iexists item
      ipureintro
      simpa [slice.slice] using Hrest
    · ihave Hglobals := cursor_initialized_bigInt_globals (GF := GF) $$ []
      · solve_pkg_init
      icases Hglobals with ⟨%k, #Hglobals⟩
      ihave Herrors := cursor_initialized_errors (GF := GF) $$ []
      · solve_pkg_init
      isimp only [CursorInitialized, CursorErrorGlobal] at Herrors
      icases Herrors with ⟨_, _, _, ⟨%depthKind, #Hdepth, _⟩, _⟩
      wp_apply wp_decodeIndexesFirst_rejected validator dst input wire retained dq consumed offset extraDepth reason k depthKind failure
        $$ [Hdst Hs] as %result ⟨Hdst, Hs, Herr, %Hoffset⟩
      · iframe Hglobals Hdepth Hdst Hs
      iapply HΦ
      iframe Hs
      iexists IndexesFirstResult.failure indexesZeroValue (.decode offset reason)
      isplitl []
      · ipureintro; exact .rejected failure
      isimp only [IndexesFirstResultOwned, IndexesAPIErrorOwned]
      iframe Hdst
      isplitl [Herr]
      · iexists k, depthKind
        iframe Hglobals Hdepth Herr
        ipureintro; exact Hoffset.1
      ipureintro; trivial
  · have hne : h ≠ zero_val BlockTransactionsIndexes := fun hz => zero (Hzero.mp hz)
    ihave Hobjects := entryErrors_initialized_objects (GF := GF) $$ []
    · solve_pkg_init
    isimp only [EntryErrorsInitialized, errors.newVar] at Hobjects
    icases Hobjects with ⟨⟨%unsupported, #Hkind, _⟩, _⟩
    wp_apply wp_decodeIndexesFirst_nonzero dst h input _ hne $$ [Hdst] as %err ⟨Hdst, Herr⟩
    · iframe Hkind Hdst
    iapply HΦ
    iframe Hs
    iexists IndexesFirstResult.failure old .nonzero
    isplitl []
    · ipureintro; exact .nonzero old wire zero
    isimp only [IndexesFirstResultOwned, IndexesAPIErrorOwned]
    isplitl [Hdst Hchildren]
    · unfold IndexesOwned
      iexists h
      iframe Hdst Hchildren
    isplitl [Herr]
    · iexists _, err
      iframe Hkind Herr
      ipureintro; trivial
    ipureintro; trivial

/-- Actual full decoding for every input and owned destination, including guard,
    parser and trailing-data errors, complete cleanup and unchanged input. -/
theorem wp_decodeIndexes_complete (validator : Utf8ValidatorContract (GF := GF))
    (dst : Loc) (old : IndexesValue) (input : GoSlice) (wire : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       IndexesOwned dst old ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeBlockTransactionsIndexes)) (Val #input)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       input ↦*{dq} wire ∗ ∃ outcome : IndexesDecodeResult,
       ⌜IndexesFullSpec old wire outcome⌝ ∗ IndexesFullResultOwned dst result outcome }} := by
  iintro %Φ ⟨#Hpkg, #HentryPkg, Howned, Hs⟩ HΦ
  isimp only [IndexesOwned] at Howned
  icases Howned with ⟨%h, Hdst, Hchildren⟩
  ihave %Hzero := indexesChildren_zero_iff h old $$ Hchildren
  by_cases zero : old = indexesZeroValue
  · subst old
    have hz := Hzero.mpr rfl
    subst h
    rcases indexesRecord_classify 0 indexesZeroValue wire (by decide) with
      ⟨item, rest, final, split, valid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
    · subst wire
      by_cases empty : rest = []
      · subst rest
        isimp only [List.append_nil] at Hs HΦ
        wp_apply wp_decodeIndexes_valid validator dst input item final dq valid $$ [Hdst Hs] with ⟨Hdst, Hs⟩
        · iframe Hdst Hs
        iapply HΦ
        iframe Hs
        iexists IndexesDecodeResult.success final
        isplitl []
        · ipureintro; exact .success (by simpa only [List.append_nil] using IndexesFirstSpec.success (rest := []) valid)
        isimp only [IndexesFullResultOwned]
        iframe Hdst
      · ihave Hobjects := entryErrors_initialized_objects (GF := GF) $$ []
        · solve_pkg_init
        isimp only [EntryErrorsInitialized, errors.newVar] at Hobjects
        icases Hobjects with ⟨_, ⟨%extra, #Hextra, _⟩⟩
        wp_apply wp_decodeIndexes_valid_trailing validator dst input item rest final dq _ valid empty
          $$ [Hdst Hs] as %err ⟨Hdst, Hs, Herr⟩
        · iframe Hextra Hdst Hs
        iapply HΦ
        iframe Hs
        iexists IndexesDecodeResult.failure indexesZeroValue (.trailing rest.length)
        isplitl []
        · ipureintro; exact .trailing (.success valid) empty
        isimp only [IndexesFullResultOwned, IndexesAPIErrorOwned]
        iframe Hdst
        iexists _, err
        iframe Hextra Herr
        ipureintro; trivial
    · ihave Hglobals := cursor_initialized_bigInt_globals (GF := GF) $$ []
      · solve_pkg_init
      icases Hglobals with ⟨%k, #Hglobals⟩
      ihave Herrors := cursor_initialized_errors (GF := GF) $$ []
      · solve_pkg_init
      isimp only [CursorInitialized, CursorErrorGlobal] at Herrors
      icases Herrors with ⟨_, _, _, ⟨%depthKind, #Hdepth, _⟩, _⟩
      wp_apply wp_decodeIndexes_rejected validator dst input wire retained dq consumed offset extraDepth reason k depthKind failure
        $$ [Hdst Hs] as %result ⟨Hdst, Hs, Herr, %Hoffset⟩
      · iframe Hglobals Hdepth Hdst Hs
      iapply HΦ
      iframe Hs
      iexists IndexesDecodeResult.failure indexesZeroValue (.decode offset reason)
      isplitl []
      · ipureintro; exact .firstError (.rejected failure)
      isimp only [IndexesFullResultOwned, IndexesAPIErrorOwned]
      iframe Hdst
      iexists k, depthKind
      iframe Hglobals Hdepth Herr
      ipureintro; exact Hoffset.1
  · have hne : h ≠ zero_val BlockTransactionsIndexes := fun hz => zero (Hzero.mp hz)
    ihave Hobjects := entryErrors_initialized_objects (GF := GF) $$ []
    · solve_pkg_init
    isimp only [EntryErrorsInitialized, errors.newVar] at Hobjects
    icases Hobjects with ⟨⟨%unsupported, #Hkind, _⟩, _⟩
    wp_apply wp_decodeIndexes_nonzero dst h input _ hne $$ [Hdst] as %err ⟨Hdst, Herr⟩
    · iframe Hkind Hdst
    iapply HΦ
    iframe Hs
    iexists IndexesDecodeResult.failure old .nonzero
    isplitl []
    · ipureintro; exact .firstError (.nonzero old wire zero)
    isimp only [IndexesFullResultOwned, IndexesAPIErrorOwned]
    isplitl [Hdst Hchildren]
    · unfold IndexesOwned
      iexists h
      iframe Hdst Hchildren
    iexists _, err
    iframe Hkind Herr
    ipureintro; trivial

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeIndexesFirst_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeIndexes_complete
