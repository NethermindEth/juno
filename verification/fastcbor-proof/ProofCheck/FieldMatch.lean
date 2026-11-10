import ProofCheck.FieldFold

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 1600000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

/-- A successful lookup names a matching field; a miss excludes every field.
The generated tables have distinct folded names, making a successful index unique. -/
def FieldMatchResult (names : List GoString) (key : List w8) (index : w64) (ok : Bool) : Prop :=
  if ok then 0 ≤ sint.Z index ∧ ∃ name,
    names[(sint.Z index).toNat]? = some name ∧ key.map fieldFoldByte = name.map fieldFoldByte
  else index = W64 0 ∧ ∀ name ∈ names, key.map fieldFoldByte ≠ name.map fieldFoldByte

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Both lookup passes preserve fractional metadata and the caller's input. -/
theorem wp_fields_match (fields : Loc) (f : cursor.Fields) (names : List GoString)
    (key : GoSlice) (xs : List w8) (qf qn dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       fields ↦{qf} f ∗ f.names' ↦*{qn} names ∗ key ↦*{dq} xs }}
      (App (Val (fields @!! go.PointerType cursor.Fields.ty @!! go!"match")) (Val #key))
    {{ (index : w64) (ok : Bool), RET (PairV #index #ok);
       fields ↦{qf} f ∗ f.names' ↦*{qn} names ∗ key ↦*{dq} xs ∗
       ⌜FieldMatchResult names xs index ok⌝ }} := by
  wp_start as ⟨Hfields, Hnames, Hkey⟩
  ihave %Hnameslen := ownSlice_len _ _ _ $$ Hnames
  ihave %Hkeylen := ownSlice_len _ _ _ $$ Hkey
  icases (cursor.Fields.Fields_access_load_names fields f qf).access_strict $$ Hfields with ⟨Hfn, Hrestore⟩
  wp_auto
  rename_i field_i_ptr
  irename : (field_i_ptr ↦ zero_val w64 : IProp GF) => Hfield
  ihave HI : (∃ (n : Nat) (oldi : w64) (oldname : GoString),
      "Hrange" ∷ i_ptr ↦ W64 n ∗ "Hfield" ∷ field_i_ptr ↦ oldi ∗ "Hname" ∷ name_ptr ↦ oldname ∗
      "%Hbound" ∷ ⌜n ≤ names.length⌝ : IProp GF) $$ [i Hfield name]
  · iexists 0, (W64 0), (go!"")
    iframe
    ipureintro; omega
  wp_for HI
  by_cases hmore : n < names.length
  · have hcond : sint.Z (W64 n) < sint.Z f.names'.len := by word
    simp only [hcond, _root_.decide_true, ite_true]
    wp_auto
    have hi : sint.Z (W64 n) = n := by word
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by word, hcond⟩)]
    rw [hi]
    let name := names[n]'hmore
    have hlookup : names[n]? = some name := List.getElem?_eq_getElem hmore
    wp_apply wp_load_slice_index f.names' n names qn name (by omega) $$ [Hnames] with Hnames
    · iframe Hnames; ipureintro; simpa using hlookup
    wp_apply wp_bytes_to_string key xs dq $$ [Hkey] with Hkey
    · iframe Hkey
    by_cases heq : xs = name
    · simp only [heq, _root_.decide_true]
      wp_auto
      wp_for_post
      ihave Hfields := Hrestore $$ Hfn
      iapply HΦ
      iframe Hfields Hnames Hkey
      ipureintro
      simp only [FieldMatchResult, ite_true, hi, Int.toNat_natCast]
      exact ⟨by omega, name, hlookup, rfl⟩
    · simp only [heq, _root_.decide_false]
      wp_auto
      wp_for_post
      have hinc : W64 n + W64 1 = W64 (n + 1) := by word
      isimp only [hinc] at Hrange
      iframe
      iexists (n + 1), (W64 n), name
      iframe
      ipureintro; omega
  · have hcond : ¬ sint.Z (W64 n) < sint.Z f.names'.len := by word
    simp only [hcond, _root_.decide_false, Bool.false_eq_true, ite_false]
    wp_auto
    rename_i field2_ptr
    irename : (field2_ptr ↦ zero_val w64 : IProp GF) => Hfield2
    ihave HI2 : (∃ (k : Nat) (oldi : w64) (oldname : GoString),
        "Hrange2" ∷ i_ptr ↦ W64 k ∗ "Hfield2" ∷ field2_ptr ↦ oldi ∗ "Hname2" ∷ name_ptr ↦ oldname ∗
        "%Hbound2" ∷ ⌜k ≤ names.length⌝ ∗
        "%Hmiss" ∷ ⌜∀ j, j < k → ∀ name, names[j]? = some name → xs.map fieldFoldByte ≠ name.map fieldFoldByte⌝ : IProp GF)
        $$ [i Hfield2 name]
    · iexists 0, (W64 0), (go!"")
      iframe
      ipureintro
      exact ⟨Nat.zero_le _, fun j hj => False.elim (by omega)⟩
    wp_for HI2
    by_cases hmore2 : k < names.length
    · have hcond2 : sint.Z (W64 k) < sint.Z f.names'.len := by word
      simp only [hcond2, _root_.decide_true, ite_true]
      wp_auto
      have hi : sint.Z (W64 k) = k := by word
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by word, hcond2⟩)]
      rw [hi]
      let name := names[k]'hmore2
      have hlookup : names[k]? = some name := List.getElem?_eq_getElem hmore2
      wp_apply wp_load_slice_index f.names' k names qn name (by omega) $$ [Hnames] with Hnames
      · iframe Hnames; ipureintro; simpa using hlookup
      wp_apply github_com.mit_pdos.perennial.goose.model.strings.wp_string_len with %Hnamebound
      by_cases hlen : xs.length = name.length
      · have heqlen : key.len = W64 name.length := by word
        simp only [heqlen, _root_.decide_true]
        wp_auto
        wp_apply wp_equalFoldASCII key xs name dq hlen $$ [Hkey] as %ok ⟨Hkey, %Hmatch⟩
        · iframe Hkey
        cases ok with
        | false =>
          wp_auto
          have hmiss : xs.map fieldFoldByte ≠ name.map fieldFoldByte := by
            intro h; exact Bool.noConfusion (Hmatch.mpr h)
          wp_for_post
          have hinc : W64 k + W64 1 = W64 (k + 1) := by word
          isimp only [hinc] at Hrange2
          iframe
          iexists (k + 1), (W64 k), name
          iframe
          ipureintro
          refine ⟨by omega, ?_⟩
          intro j hj other hother
          by_cases hjk : j < k
          · exact Hmiss j hjk other hother
          · have hjk : j = k := by omega
            subst j
            have hother' : other = name := Option.some.inj (hother.symm.trans hlookup)
            subst other
            exact hmiss
        | true =>
          wp_auto
          wp_for_post
          ihave Hfields := Hrestore $$ Hfn
          iapply HΦ
          iframe Hfields Hnames Hkey
          ipureintro
          simp only [FieldMatchResult, ite_true, hi, Int.toNat_natCast]
          exact ⟨by omega, name, hlookup, Hmatch.mp rfl⟩
      · have hneqlen : key.len ≠ W64 name.length := by word
        simp only [hneqlen, _root_.decide_false]
        wp_auto
        have hmiss : xs.map fieldFoldByte ≠ name.map fieldFoldByte := by
          intro h
          have hlength := congrArg List.length h
          simp only [List.length_map] at hlength
          exact hlen hlength
        wp_for_post
        have hinc : W64 k + W64 1 = W64 (k + 1) := by word
        isimp only [hinc] at Hrange2
        iframe
        iexists (k + 1), (W64 k), name
        iframe
        ipureintro
        refine ⟨by omega, ?_⟩
        intro j hj other hother
        by_cases hjk : j < k
        · exact Hmiss j hjk other hother
        · have hjk : j = k := by omega
          subst j
          have hother' : other = name := Option.some.inj (hother.symm.trans hlookup)
          subst other
          exact hmiss
    · have hcond2 : ¬ sint.Z (W64 k) < sint.Z f.names'.len := by word
      simp only [hcond2, _root_.decide_false, Bool.false_eq_true, ite_false]
      wp_auto
      ihave Hfields := Hrestore $$ Hfn
      iapply HΦ
      iframe Hfields Hnames Hkey
      ipureintro
      simp only [FieldMatchResult]
      refine ⟨True.intro, ?_⟩
      intro name hmem
      obtain ⟨j, hj⟩ := List.mem_iff_getElem?.mp hmem
      have hbound := (List.getElem?_eq_some_iff.mp hj).choose
      exact Hmiss j (by omega) name hj


/-- Byte folding is ordinary ASCII arithmetic; no wrapping occurs in the uppercase branch. -/
theorem fieldFoldByte_value (b : w8) :
    uint.Z (fieldFoldByte b) = if 65 ≤ uint.Z b ∧ uint.Z b ≤ 90 then uint.Z b + 32 else uint.Z b := by
  unfold fieldFoldByte
  split <;> word

/-- Distinct folded names make the successful index unique, independently of lookup order. -/
theorem fieldMatchResult_unique (names : List GoString) (key : List w8) (i j : w64)
    (hdistinct : (names.map (List.map fieldFoldByte)).Nodup)
    (hi : FieldMatchResult names key i true) (hj : FieldMatchResult names key j true) : i = j := by
  rcases hi with ⟨hi0, ni, hni, hi⟩
  rcases hj with ⟨hj0, nj, hnj, hj⟩
  have hbound := (List.getElem?_eq_some_iff.mp hni).choose
  have heq : (names.map (List.map fieldFoldByte))[(sint.Z i).toNat]? =
      (names.map (List.map fieldFoldByte))[(sint.Z j).toNat]? := by
    simp only [List.getElem?_map, hni, hnj, Option.map_some]
    rw [← hi, ← hj]
  have hn : (sint.Z i).toNat = (sint.Z j).toNat :=
    (hdistinct.getElem?_inj (by simpa only [List.length_map] using hbound)).mp heq
  word

/-- Folded Header field names are distinct, so a successful lookup cannot name two fields. -/
theorem headerFoldedNames_distinct : (headerFieldNames.map (List.map fieldFoldByte)).Nodup := by decide

/-- The same uniqueness property for the nested GasPrice schema. -/
theorem gasFoldedNames_distinct : (gasFieldNames.map (List.map fieldFoldByte)).Nodup := by decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_fields_match

#print axioms Perennial.github_com.NethermindEth.juno.core.headerFoldedNames_distinct

#print axioms Perennial.github_com.NethermindEth.juno.core.gasFoldedNames_distinct

#print axioms Perennial.github_com.NethermindEth.juno.core.fieldFoldByte_value

#print axioms Perennial.github_com.NethermindEth.juno.core.fieldMatchResult_unique
