local f=game.forces["player"] local s=game.surfaces[1] local r={}
r.tick=game.tick
r.players=#game.connected_players
r.evolution=game.forces["enemy"].get_evolution_factor(s)
r.item_production=f.item_production_statistics.input_counts
r.item_consumption=f.item_production_statistics.output_counts
r.fluid_production=f.fluid_production_statistics.input_counts
r.fluid_consumption=f.fluid_production_statistics.output_counts
r.kill_counts=f.kill_count_statistics.input_counts
r.entity_built=f.entity_build_count_statistics.input_counts
r.rockets_launched=f.rockets_launched
r.research=f.current_research and f.current_research.name or nil
r.research_progress=f.research_progress
local poles=s.find_entities_filtered{type="electric-pole",limit=1}
if poles[1] then
  r.power_production=poles[1].electric_network_statistics.input_counts
  r.power_consumption=poles[1].electric_network_statistics.output_counts
end
rcon.print(game.table_to_json(r))
