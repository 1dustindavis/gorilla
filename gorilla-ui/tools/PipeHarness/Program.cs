using Gorilla.UI.Client;

Console.WriteLine("Gorilla Pipe Harness (scaffold)");
Console.WriteLine("Commands:");
Console.WriteLine("  list");
Console.WriteLine("  install <itemName>");
Console.WriteLine("  remove <itemName>");
Console.WriteLine("  stream <operationId>");
Console.WriteLine();
Console.WriteLine("Implementation note: wire this CLI to a NamedPipeGorillaServiceClient once the service envelope is finalized.");
