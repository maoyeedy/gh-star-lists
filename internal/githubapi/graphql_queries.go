package githubapi

const listStarListsQuery = `query($endCursor: String, $first: Int!) {
  viewer {
    lists(first: $first, after: $endCursor) {
      nodes {
        id
        name
        slug
        description
        lastAddedAt
        isPrivate
        items {
          totalCount
        }
        user {
          login
        }
      }
      pageInfo {
        hasNextPage
        endCursor
      }
    }
  }
}`

// repositoryFieldsFragment is the shared set of repository fields fetched by all three
// repository queries. Embedded via Go const concatenation; GraphQL is whitespace-insensitive.
// repositoryTopics uses @include(if: $withTopics); callers must declare $withTopics: Boolean!.
const repositoryFieldsFragment = `
    id
    nameWithOwner
    description
    url
    isFork
    isArchived
    stargazerCount
    pushedAt
    licenseInfo {
      key
    }
    repositoryTopics(first: 20) @include(if: $withTopics) {
      nodes {
        topic {
          name
        }
      }
    }
    primaryLanguage {
      name
    }`

const listStarredRepositoriesQuery = `query($endCursor: String, $first: Int!, $withTopics: Boolean!) {
  viewer {
    starredRepositories(first: $first, after: $endCursor, orderBy: {field: STARRED_AT, direction: DESC}) {
      edges {
        starredAt
        node {` + repositoryFieldsFragment + `
        }
      }
      pageInfo {
        hasNextPage
        endCursor
      }
    }
  }
}`

const getRepositoryQuery = `query($owner: String!, $name: String!, $withTopics: Boolean!) {
  repository(owner: $owner, name: $name) {` + repositoryFieldsFragment + `
  }
}`

const getRepositoryIDQuery = `query($owner: String!, $name: String!) {
  repository(owner: $owner, name: $name) {
    id
    nameWithOwner
  }
}`

const createStarListMutation = `mutation($name: String!, $description: String, $private: Boolean!) {
  createUserList(input: {name: $name, description: $description, isPrivate: $private}) {
    list {
      id
      name
      slug
      description
      lastAddedAt
      items {
        totalCount
      }
      user {
        login
      }
    }
  }
}`

const updateStarListMutation = `mutation($listID: ID!, $name: String, $description: String, $private: Boolean) {
  updateUserList(input: {listId: $listID, name: $name, description: $description, isPrivate: $private}) {
    list {
      id
      name
      slug
      description
      lastAddedAt
      items {
        totalCount
      }
      user {
        login
      }
    }
  }
}`

const deleteStarListMutation = `mutation($listID: ID!) {
  deleteUserList(input: {listId: $listID}) {
    clientMutationId
  }
}`

const updateRepositoryListsMutation = `mutation($itemID: ID!, $listIDs: [ID!]!) {
  updateUserListsForItem(input: {itemId: $itemID, listIds: $listIDs}) {
    clientMutationId
  }
}`

const addStarMutation = `mutation($starrableID: ID!) {
  addStar(input: {starrableId: $starrableID}) {
    clientMutationId
  }
}`

const removeStarMutation = `mutation($starrableID: ID!) {
  removeStar(input: {starrableId: $starrableID}) {
    clientMutationId
  }
}`

const listRepositoriesQuery = `query($id: ID!, $endCursor: String, $first: Int!, $withTopics: Boolean!) {
  node(id: $id) {
    __typename
    ... on UserList {
      items(first: $first, after: $endCursor) {
        nodes {
          __typename
          ... on Repository {` + repositoryFieldsFragment + `
          }
        }
        pageInfo {
          hasNextPage
          endCursor
        }
      }
    }
  }
}`
