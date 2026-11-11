import ProofCheck.MigratedClassesUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Unlike nullable pointer maps, the migrated-class map rejects null/undefined
    values after consuming the key. The retained key is not a published entry. -/
theorem migratedClassesEntry_nullish_value (d : Nat) (item rest : List w8)
    (key : FeltMapKey) (head : w8) (valid : FeltMapKeyValid d item key)
    (hn : head = W8 246 ∨ head = W8 247) :
    MigratedClassesEntryRejected d (item ++ head :: rest) (some key)
      item.length item.length false (.value (.expected (W8 128) (.typeMismatch head))) := by
  simpa only [Nat.add_zero] using MigratedClassesEntryRejected.value valid
    (feltMapKey_nullish_rejected d head rest hn)

theorem migratedClassesEntry_nullish_key (d : Nat) (rest : List w8)
    (head : w8) (hn : head = W8 246 ∨ head = W8 247) :
    MigratedClassesEntryRejected d (head :: rest) none 0 0 false
      (.key (.expected (W8 128) (.typeMismatch head))) :=
  .key (feltMapKey_nullish_rejected d head rest hn)

/-- Every width encoding zero pairs produces a present empty map, not nil. -/
theorem migratedClassesSpec_empty (d : Nat) (head : w8) (payload rest : List w8)
    (enc : HeadArgument head payload (W64 0)) (major : head &&& W8 224 = W8 160)
    (depth : d < 32) :
    MigratedClassesSpec d ((head :: payload) ++ rest) (.success (some ∅) rest) := by
  simpa only [List.append_nil, migratedClassesOfEntries, migratedClassesApply, List.foldl_nil] using
    (MigratedClassesSpec.map (count := 0) (item := []) enc major (by rfl)
      (by simp) depth (.nil rest))

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClassesEntry_nullish_value
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClassesEntry_nullish_key
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClassesSpec_empty
