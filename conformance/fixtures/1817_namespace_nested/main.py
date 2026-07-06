# Namespace packages nest: neither org nor org.team has an __init__.py, so both
# are bodyless namespace modules with a None __file__, and the leaf module
# under them resolves and executes normally.
import org.team.task

print("org file:", org.__file__)
print("org.team file:", org.team.__file__)
print("org type:", type(org).__name__)
print("task:", org.team.task.job)
