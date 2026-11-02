namespace Evoq.Surfdude.Hypertext
{
    public interface IHypertextInputControl
    {
        string Name { get; }

        bool IsOptional { get; }

        /// <summary>Suggested or server-known value for this input, or null when the API did not supply one.</summary>
        string Value { get; }
    }
}