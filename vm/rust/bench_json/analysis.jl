import CSV 
using DataFrames
using Plots
using Measures
using Statistics

cmd = read(`./extract.sh`, String)
df = CSV.read(IOBuffer(cmd), DataFrame, header=false)
df.speedup = df[:,3] ./ df[:,2]

# Settings to show x86 data
#histogram(df.speedup, size = (1250,650), x_ticks = 0:0.5:20, xlims =(0,18.5), y_ticks = 0:1:30, ylims=(0,30), xlabel = "speed up factor", ylabel="count", title="Native performance x86-64", margin=5mm)
#savefig("x86_64.svg")

# TODO Set ticks and limits based on the data you receive
histogram(df.speedup, size = (1250,650), xlabel = "speed up factor", ylabel="count", title="Native performance arm", margin=5mm)
savefig("arm.svg")

println("Mean speed up: ", mean(df.speedup))
println("Median speed up: ", median(df.speedup))

# To keep the batch script easy we didn't extract the block name so we do it here
df[:,1] = map(x -> match(r"\d+", x).match, df[:,1])
sorted_df = sort(df, :speedup)

# worst performing
bottom = first(sorted_df, 5)
println(bottom)
println(join(bottom[:,1], " "))

# best performing
top = last(sorted_df, 5)
println(top)
println(join(top[:,1], " "))
