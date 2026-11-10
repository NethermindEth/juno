import ProofCheck.GasPriceFieldComplete

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

/-- A unique folded-name match determines the field identity even when the
    lookup used a cache miss or a different spelling. Duplicate handling remains
    the actual reader's disjunction, rather than an assumed fresh-field result. -/
theorem fieldReadOutcome_known (st : Loc) (s : cursor.Struct) (names : List GoString)
    (key name : GoString) (index code : w64) (err kind : GoInterface) (offset : w64)
    (distinct : (names.map (List.map fieldFoldByte)).Nodup)
    (hi : 0 ≤ sint.Z index) (lookup : names[(sint.Z index).toNat]? = some name)
    (hmatch : key.map fieldFoldByte = name.map fieldFoldByte) :
    FieldReadOutcome st s names key code err kind offset ⊢
      (KnownFieldOutcome st s index name code err kind offset : IProp GF) := by
  unfold FieldReadOutcome
  iintro Hout
  icases Hout with (Hmiss | Hfound)
  · icases Hmiss with ⟨%Hmiss, -⟩
    exfalso
    exact Hmiss.2.2 name (List.mem_of_getElem? lookup) hmatch
  · icases Hfound with ⟨%field, %foundName, %Hmatch, Hfound⟩
    have heq : field = index := fieldMatchResult_unique names key field index distinct
      ⟨Hmatch.1, foundName, Hmatch.2.2.1, Hmatch.2.2.2⟩ ⟨hi, name, lookup, hmatch⟩
    subst field
    have hname : foundName = name := Option.some.inj (Hmatch.2.2.1.symm.trans lookup)
    subst foundName
    unfold KnownFieldOutcome
    iexact Hfound

/-- Excluding every folded schema name determines an unknown-key result. -/
theorem fieldReadOutcome_unknown (st : Loc) (s : cursor.Struct) (names : List GoString)
    (key : GoString) (code : w64) (err kind : GoInterface) (offset : w64)
    (missing : ∀ name ∈ names, key.map fieldFoldByte ≠ name.map fieldFoldByte) :
    FieldReadOutcome st s names key code err kind offset ⊢
      (iprop(⌜code = W64 (-2) ∧ err = interface.nil⌝ ∗ st ↦ s) : IProp GF) := by
  unfold FieldReadOutcome
  iintro Hout
  icases Hout with (Hmiss | Hfound)
  · icases Hmiss with ⟨%Hmiss, Hst⟩
    iframe Hst
    ipureintro; exact ⟨Hmiss.1, Hmiss.2.1⟩
  · icases Hfound with ⟨%field, %name, %Hmatch, -⟩
    exfalso
    exact missing name (List.mem_of_getElem? Hmatch.2.2.1) Hmatch.2.2.2

/-- A fresh known field has exactly the seen-bit update specified by its schema. -/
theorem knownFieldOutcome_fresh (st : Loc) (s : cursor.Struct) (index : w64) (name : GoString)
    (code : w64) (err kind : GoInterface) (offset : w64)
    (fresh : s.seen' &&& (W64 1 <<< index) = W64 0) :
    KnownFieldOutcome st s index name code err kind offset ⊢
      (iprop(⌜code = index ∧ err = interface.nil⌝ ∗ st ↦ {s with seen' := s.seen' ||| (W64 1 <<< index)}) : IProp GF) := by
  unfold KnownFieldOutcome
  iintro Hout
  icases Hout with (Hnew | Hduplicate)
  · icases Hnew with ⟨%Hnew, Hst⟩
    iframe Hst
    ipureintro; exact ⟨Hnew.1, Hnew.2.1⟩
  · icases Hduplicate with ⟨%errLoc, %args, %Hduplicate, -⟩
    exfalso; exact Hduplicate.2.2 fresh

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.fieldReadOutcome_known
#print axioms Perennial.github_com.NethermindEth.juno.core.fieldReadOutcome_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.knownFieldOutcome_fresh
